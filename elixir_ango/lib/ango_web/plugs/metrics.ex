defmodule AngoWeb.Plugs.Metrics do
  @moduledoc """
  Plug for collecting HTTP request metrics.
  
  Measures request duration, throughput, and status codes
  for monitoring and alerting purposes.
  """

  import Plug.Conn
  require Logger

  def init(opts), do: opts

  def call(conn, _opts) do
    start_time = System.monotonic_time()

    register_before_send(conn, fn conn ->
      duration = System.monotonic_time() - start_time
      record_request_metrics(conn, duration)
      conn
    end)
  end

  defp record_request_metrics(conn, duration_ns) do
    :telemetry.execute(
      [:ango_web, :request, :stop],
      %{duration: duration_ns},
      %{
        method: conn.method,
        path: conn.request_path,
        status: conn.status
      }
    )
  end
end