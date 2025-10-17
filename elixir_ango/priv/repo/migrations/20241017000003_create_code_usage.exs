defmodule Ango.Repo.Migrations.CreateCodeUsage do
  use Ecto.Migration

  def change do
    create table(:code_usage) do
      add :code, :string, null: false
      add :batch_id, :binary_id, null: false
      add :client_id, :binary_id, null: false
      add :customer_id, :binary_id, null: false
      add :used_at, :utc_datetime, null: false

      timestamps()
    end

    create index(:code_usage, [:customer_id, :used_at])
    create index(:code_usage, [:batch_id, :used_at])
    create index(:code_usage, [:used_at])
  end
end