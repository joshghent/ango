defmodule Ango.Repo.Migrations.CreateCodes do
  use Ecto.Migration

  def change do
    create table(:codes) do
      add :code, :string, null: false
      add :batch_id, :binary_id, null: false
      add :client_id, :string, null: false
      add :customer_id, :binary_id
      add :used_at, :utc_datetime
      add :invalid_reason, :string

      timestamps()
    end

    # Critical indexes for performance (matching Go version)
    create unique_index(:codes, [:code])
    
    # Sub-10ms code lookup - optimized for unredeemed codes
    create index(:codes, [:batch_id, :client_id, :id], 
      where: "customer_id IS NULL",
      name: :idx_codes_batch_client_unredeemed)
    
    # Fast rule validation - optimized for redeemed codes
    create index(:codes, [:customer_id, :id], 
      where: "customer_id IS NOT NULL",
      name: :idx_codes_customer_used)
    
    # Foreign key constraint
    create constraint(:codes, :batch_id_exists, 
      foreign_key: [batch_id: :batches])
  end
end