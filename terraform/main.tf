resource "digitalocean_app" "ango" {
  spec {
    name   = "ango"
    region = "lon1"

    service {
      name = "app"
      git {
        repo_clone_url = "https://github.com/joshghent/ango.git"
        branch         = "main"
      }
      dockerfile_path    = "Dockerfile"
      instance_size_slug = "basic-xxs" # Choose your instance size
      instance_count     = 1

      # env {
      #   key   = "DATABASE_URL"
      #   value = digitalocean_app.ango.database[0].db_url
      # }

      http_port = 3000
    }

    database {
      name       = "db"
      engine     = "PG"
      production = false
    }
  }
}
