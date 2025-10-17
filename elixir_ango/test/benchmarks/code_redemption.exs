# Benchmark script for code redemption performance
# Run with: mix run test/benchmarks/code_redemption.exs

Mix.install([
  {:benchee, "~> 1.0"}
])

# Setup test environment
Application.ensure_all_started(:ango)

alias Ango.{Repo, Schemas.Code, Schemas.Batch}

# Create test data
batch = %Batch{
  id: UUID.uuid4(),
  name: "Benchmark Batch",
  rules: %{"maxpercustomer" => 1000, "timelimit" => 30},
  expired: false
} |> Repo.insert!()

client_id = UUID.uuid4()

# Create 10,000 test codes
IO.puts("Creating 10,000 test codes...")

codes = for i <- 1..10_000 do
  %{
    code: "BENCH-#{String.pad_leading("#{i}", 6, "0")}",
    batch_id: batch.id,
    client_id: client_id,
    inserted_at: DateTime.utc_now(),
    updated_at: DateTime.utc_now()
  }
end

{count, _} = Repo.insert_all(Code, codes)
IO.puts("Created #{count} codes for benchmarking")

# Benchmark code redemption
Benchee.run(
  %{
    "code_redemption" => fn ->
      customer_id = UUID.uuid4()
      
      case Ango.Codes.get_code(batch.id, client_id, customer_id) do
        {:ok, _code} -> :success
        {:error, _reason} -> :error
      end
    end,
    
    "batch_rules_lookup" => fn ->
      Ango.Batches.get_batch_rules(batch.id)
    end,
    
    "code_count_check" => fn ->
      customer_id = UUID.uuid4()
      Ango.Codes.get_code_count(customer_id, 30)
    end
  },
  time: 10,
  memory_time: 2,
  formatters: [
    Benchee.Formatters.HTML,
    Benchee.Formatters.Console
  ],
  formatter_options: [html: [file: "benchmark_results.html"]]
)

IO.puts("""

🎯 Benchmark Results Summary:

The benchmarks measure:
1. Code redemption performance (end-to-end)
2. Batch rules lookup (with caching)
3. Customer code count checking

Key metrics to monitor:
- Average execution time < 10ms
- Memory usage stays reasonable
- No significant performance degradation

Results saved to benchmark_results.html
""")