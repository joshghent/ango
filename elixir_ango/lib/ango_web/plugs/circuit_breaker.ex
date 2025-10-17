defmodule AngoWeb.Plugs.CircuitBreaker do
  @moduledoc """
  Plug for circuit breaker protection.
  
  Rejects requests early if critical circuit breakers are open,
  preventing cascade failures.
  """

  import Plug.Conn
  alias Ango.CircuitBreaker

  def init(opts), do: opts

  def call(conn, _opts) do
    case CircuitBreaker.get_state() do
      %{database: :open} ->
        conn
        |> put_status(:service_unavailable)
        |> put_resp_content_type("application/json")
        |> send_resp(503, Jason.encode!(%{
          error: "Service temporarily unavailable",
          message: "Database is experiencing issues. Please try again later.",
          retry_after: 30
        }))
        |> halt()

      _ ->
        conn
    end
  end
end