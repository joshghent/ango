defmodule Ango.Repo.Migrations.CreateBatches do
  use Ecto.Migration

  def change do
    create table(:batches, primary_key: false) do
      add :id, :binary_id, primary_key: true
      add :name, :string, null: false
      add :rules, :map
      add :expired, :boolean, default: false, null: false

      timestamps()
    end

    create index(:batches, [:expired])
    create unique_index(:batches, [:name])
  end
end