defmodule AngoWeb.MetricsController do
  use AngoWeb, :controller

  @doc """
  Prometheus metrics endpoint.
  
  Returns metrics in Prometheus text format for scraping.
  """
  def prometheus(conn, _params) do
    metrics = collect_prometheus_metrics()
    
    conn
    |> put_resp_content_type("text/plain")
    |> send_resp(200, metrics)
  end

  # Private functions

  defp collect_prometheus_metrics do
    """
    # HELP ango_requests_total Total HTTP requests
    # TYPE ango_requests_total counter
    ango_requests_total{method="POST",endpoint="/api/v1/code/redeem",status="200"} #{get_metric_value(:requests_total, :post_redeem_success)}
    ango_requests_total{method="POST",endpoint="/api/v1/code/redeem",status="400"} #{get_metric_value(:requests_total, :post_redeem_error)}
    ango_requests_total{method="GET",endpoint="/api/v1/batches",status="200"} #{get_metric_value(:requests_total, :get_batches_success)}

    # HELP ango_request_duration_seconds HTTP request duration in seconds
    # TYPE ango_request_duration_seconds histogram
    ango_request_duration_seconds_bucket{method="POST",endpoint="/api/v1/code/redeem",le="0.1"} #{get_metric_value(:request_duration, :post_redeem_100ms)}
    ango_request_duration_seconds_bucket{method="POST",endpoint="/api/v1/code/redeem",le="0.5"} #{get_metric_value(:request_duration, :post_redeem_500ms)}
    ango_request_duration_seconds_bucket{method="POST",endpoint="/api/v1/code/redeem",le="+Inf"} #{get_metric_value(:request_duration, :post_redeem_inf)}

    # HELP ango_codes_redeemed_total Total codes redeemed
    # TYPE ango_codes_redeemed_total counter
    ango_codes_redeemed_total #{get_metric_value(:codes_redeemed, :total)}

    # HELP ango_code_redemption_errors_total Total code redemption errors
    # TYPE ango_code_redemption_errors_total counter
    ango_code_redemption_errors_total{error_type="no_code_found"} #{get_metric_value(:redemption_errors, :no_code_found)}
    ango_code_redemption_errors_total{error_type="rule_violation"} #{get_metric_value(:redemption_errors, :rule_violation)}
    ango_code_redemption_errors_total{error_type="invalid_request"} #{get_metric_value(:redemption_errors, :invalid_request)}

    # HELP ango_circuit_breaker_state Circuit breaker state (0=closed, 1=open)
    # TYPE ango_circuit_breaker_state gauge
    ango_circuit_breaker_state{circuit="database"} #{get_circuit_breaker_state(:database)}
    ango_circuit_breaker_state{circuit="redis"} #{get_circuit_breaker_state(:redis)}

    # HELP ango_codes_remaining Codes remaining per batch
    # TYPE ango_codes_remaining gauge
    #{collect_codes_remaining_metrics()}
    """
  end

  defp get_metric_value(_metric_type, _key) do
    # In a real implementation, this would fetch from ETS tables or persistent storage
    # For now, return static values as placeholders
    :rand.uniform(1000)
  end

  defp get_circuit_breaker_state(circuit_name) do
    case Ango.CircuitBreaker.get_state() do
      %{^circuit_name => :closed} -> 0
      %{^circuit_name => :open} -> 1
      _ -> 0
    end
  end

  defp collect_codes_remaining_metrics do
    # This would query the database for actual values
    # For now, return sample metrics
    """
    ango_codes_remaining{batch_id="11111111-1111-1111-1111-111111111111",client_id="217be7c8-679c-4e08-bffc-db3451bdcdbf"} 850
    ango_codes_remaining{batch_id="22222222-2222-2222-2222-222222222222",client_id="2ee73a08-ac6f-457d-934f-dcbc61840ae6"} 742
    """
  end
end