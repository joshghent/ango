defmodule AngoWeb.Plugs.Timeout do
  @moduledoc """
  Plug for request timeout handling.
  
  Sets a timeout for HTTP requests to prevent hanging connections.
  """

  import Plug.Conn
  require Logger

  @default_timeout 5000 # 5 seconds

  def init(opts) do
    timeout = Keyword.get(opts, :timeout, @default_timeout)
    %{timeout: timeout}
  end

  def call(conn, %{timeout: timeout}) do
    task = 
      Task.async(fn ->
        receive do
          {:continue, new_conn} -> new_conn
        after
          timeout ->
            Logger.warning("Request timed out after #{timeout}ms: #{conn.method} #{conn.request_path}")
            
            conn
            |> put_status(:request_timeout)
            |> put_resp_content_type("application/json")
            |> send_resp(408, Jason.encode!(%{
              error: "Request timeout",
              message: "Request took too long to process"
            }))
            |> halt()
        end
      end)

    register_before_send(conn, fn finished_conn ->
      send(task.pid, {:continue, finished_conn})
      Task.await(task, 100)
    end)
  end
end