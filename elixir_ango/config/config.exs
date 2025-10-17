# This file is responsible for configuring your application
# and its dependencies with the aid of the Config module.
#
# This configuration file is loaded before any dependency and
# is restricted to this project.

# General application configuration
import Config

config :ango,
  ecto_repos: [Ango.Repo],
  generators: [binary_id: true]

# Configures the endpoint
config :ango, AngoWeb.Endpoint,
  url: [host: "localhost"],
  render_errors: [view: AngoWeb.ErrorView, accepts: ~w(html json), layout: false],
  pubsub_server: Ango.PubSub,
  live_view: [signing_salt: "YourSigningSalt"]

# Configures the mailer
#
# By default it uses the "Local" adapter which stores the emails
# locally. You can see the emails in your browser, at "/dev/mailbox".
#
# For production it's recommended to configure a different adapter
# at the `config/runtime.exs`.
config :ango, Ango.Mailer, adapter: Swoosh.Adapters.Local

# Swoosh API client is needed for adapters other than SMTP.
config :swoosh, :api_client, false

# Configure esbuild (the version is required)
config :esbuild,
  version: "0.17.11",
  default: [
    args:
      ~w(js/app.js --bundle --target=es2017 --outdir=../priv/static/assets --external:/fonts/* --external:/images/*),
    cd: Path.expand("../assets", __DIR__),
    env: %{"NODE_PATH" => Path.expand("../deps", __DIR__)}
  ]

# Configure tailwind (the version is required)
config :tailwind,
  version: "3.3.0",
  default: [
    args: ~w(
      --config=tailwind.config.js
      --input=css/app.css
      --output=../priv/static/assets/app.css
    ),
    cd: Path.expand("../assets", __DIR__)
  ]

# Configures Elixir's Logger
config :logger, :console,
  format: "$time $metadata[$level] $message\n",
  metadata: [:request_id]

# Use Jason for JSON parsing in Phoenix
config :phoenix, :json_library, Jason

# Redis configuration
config :ango, :redis_url, System.get_env("REDIS_URL") || "redis://localhost:6379"

# Circuit breaker configuration
config :ango, :circuit_breaker,
  database: [
    max_failures: 5,
    window_time: 10_000,
    recovery_time: 30_000
  ],
  redis: [
    max_failures: 3,
    window_time: 10_000,
    recovery_time: 15_000
  ]

# Pre-allocation configuration
config :ango, :pre_allocation,
  threshold: 10_000,
  buffer_size: 1_000,
  refill_at: 100,
  check_interval: 30_000

# Async validation configuration
config :ango, :async_validator,
  worker_count: 5,
  queue_size: 1_000,
  max_job_age: 30_000

# Import environment specific config. This must remain at the bottom
# of this file so it overrides the configuration defined above.
import_config "#{config_env()}.exs"