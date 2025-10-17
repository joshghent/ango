defmodule AngoWeb.HealthController do
  use AngoWeb, :controller
  
  alias Ango.{Repo, CircuitBreaker}

  @doc """
  Health check endpoint with dependency verification.
  
  Returns comprehensive health status including:
  - Database connectivity
  - Redis connectivity 
  - Circuit breaker states
  - System information
  """
  def check(conn, _params) do
    health_status = %{
      status: "healthy",
      timestamp: DateTime.utc_now(),
      version: Application.spec(:ango, :vsn) || "unknown",
      checks: %{}
    }

    # Check database health
    {db_status, db_check} = check_database()
    
    # Check Redis health
    {redis_status, redis_check} = check_redis()
    
    # Get circuit breaker states
    circuit_states = CircuitBreaker.get_state()

    # Determine overall health
    overall_status = 
      if db_status == :healthy do
        "healthy"
      else
        "unhealthy"
      end

    health_data = %{
      health_status 
      | status: overall_status,
        checks: %{
          database: db_check,
          redis: redis_check,
          circuit_breakers: circuit_states
        }
    }

    status_code = if overall_status == "healthy", do: :ok, else: :service_unavailable
    
    conn
    |> put_status(status_code)
    |> json(health_data)
  end

  # Private functions

  defp check_database do
    try do
      case Ecto.Adapters.SQL.query(Repo, "SELECT 1", []) do
        {:ok, _} ->
          stats = get_db_pool_stats()
          {:healthy, %{
            status: "healthy",
            connections: stats
          }}
        
        {:error, error} ->
          {:unhealthy, %{
            status: "unhealthy",
            error: inspect(error)
          }}
      end
    rescue
      error ->
        {:unhealthy, %{
          status: "unhealthy", 
          error: inspect(error)
        }}
    end
  end

  defp check_redis do
    try do
      case Redix.command(:redix, ["PING"]) do
        {:ok, "PONG"} ->
          {:healthy, %{
            status: "healthy"
          }}
        
        {:error, error} ->
          {:degraded, %{
            status: "unhealthy",
            error: inspect(error)
          }}
      end
    rescue
      error ->
        {:degraded, %{
          status: "disabled",
          error: inspect(error)
        }}
    end
  end

  defp get_db_pool_stats do
    case :sys.get_state(Repo) do
      {_config, _adapter_meta, pool_state} ->
        # Extract pool statistics if available
        %{
          pool_size: "unknown",
          available: "unknown"
        }
      
      _ ->
        %{
          pool_size: "unknown", 
          available: "unknown"
        }
    end
  end
end