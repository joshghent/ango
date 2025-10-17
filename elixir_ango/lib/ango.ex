defmodule Ango do
  @moduledoc """
  Ango - High-Performance Code Redemption Service

  A high-performance, production-ready code redemption service built with Elixir,
  designed for enterprise-scale coupon and promotional code distribution systems.

  This is a complete port of the Go version with equivalent functionality:
  - Atomic code selection with PostgreSQL FOR UPDATE SKIP LOCKED
  - Redis caching and pre-allocation buffers
  - Async rule validation with worker pools
  - Circuit breaker patterns for resilience
  - Comprehensive monitoring and metrics
  - High throughput and low latency
  """

  alias Ango.{Codes, Batches}

  @doc """
  Context for Codes domain
  """
  defdelegate get_code(batch_id, client_id, customer_id), to: Codes
  defdelegate get_code_count(customer_id, time_limit), to: Codes
  defdelegate upload_codes(file_stream, batch_id), to: Codes

  @doc """
  Context for Batches domain  
  """
  defdelegate list_batches(), to: Batches
  defdelegate get_batch(id), to: Batches
  defdelegate create_batch(attrs), to: Batches
  defdelegate get_batch_rules(batch_id), to: Batches
end