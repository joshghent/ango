import Config

# Configure your database
#
# The MIX_TEST_PARTITION environment variable can be used
# to provide built-in test partitioning in CI environment.
# Run `mix help test` for more information.
config :ango, Ango.Repo,
  username: "postgres",
  password: "postgres",
  hostname: "localhost",
  database: "ango_test#{System.get_env("MIX_TEST_PARTITION")}",
  port: 5432,
  pool: Ecto.Adapters.SQL.Sandbox,
  pool_size: 10

# We don't run a server during test. If one is required,
# you can enable the server option below.
config :ango, AngoWeb.Endpoint,
  http: [ip: {127, 0, 0, 1}, port: 4002],
  secret_key_base: "YourTestSecretKeyBaseHereMustBeAtLeast64Characters1234567890123456789012345678901234567890",
  server: false

# In test we don't send emails.
config :ango, Ango.Mailer, adapter: Swoosh.Adapters.Test

# Print only warnings and errors during test
config :logger, level: :warn

# Initialize plugs at runtime for faster test compilation
config :phoenix, :plug_init_mode, :runtime

# Disable Redis for testing
config :ango, :redis_url, nil

# Test-specific circuit breaker configuration
config :ango, :circuit_breaker,
  database: [
    max_failures: 2,
    window_time: 5_000,
    recovery_time: 10_000
  ],
  redis: [
    max_failures: 2,
    window_time: 5_000,
    recovery_time: 10_000
  ]