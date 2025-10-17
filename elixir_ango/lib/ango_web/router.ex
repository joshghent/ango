defmodule AngoWeb.Router do
  use AngoWeb, :router

  pipeline :api do
    plug :accepts, ["json"]
  end

  pipeline :browser do
    plug :accepts, ["html"]
    plug :fetch_session
    plug :fetch_live_flash
    plug :put_root_layout, {AngoWeb.LayoutView, :root}
    plug :protect_from_forgery
    plug :put_secure_browser_headers
  end

  # API routes
  scope "/api/v1", AngoWeb do
    pipe_through :api

    # Code redemption endpoint
    post "/code/redeem", CodeController, :redeem

    # Batch management
    get "/batches", BatchController, :index
    post "/codes/upload", CodeController, :upload
  end

  # Health and monitoring endpoints
  scope "/", AngoWeb do
    pipe_through :api

    get "/healthcheck", HealthController, :check
    get "/metrics", MetricsController, :prometheus
    get "/circuit-breaker", CircuitBreakerController, :status
  end

  # Development and monitoring dashboard
  if Mix.env() in [:dev, :test] do
    import Phoenix.LiveDashboard.Router

    scope "/" do
      pipe_through :browser
      live_dashboard "/dashboard", metrics: AngoWeb.Telemetry
    end
  end
end