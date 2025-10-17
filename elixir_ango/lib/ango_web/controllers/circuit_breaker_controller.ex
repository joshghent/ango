defmodule AngoWeb.CircuitBreakerController do
  use AngoWeb, :controller
  
  alias Ango.CircuitBreaker

  @doc """
  Get current circuit breaker states.
  
  Returns the state of all circuit breakers in the system.
  """
  def status(conn, _params) do
    states = CircuitBreaker.get_state()
    
    json(conn, states)
  end
end