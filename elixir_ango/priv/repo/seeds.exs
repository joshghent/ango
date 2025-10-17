# Script for populating the database. You can run it as:
#
#     mix run priv/repo/seeds.exs
#
# Inside the script, you can read and write to any of your
# repositories directly:
#
#     Ango.Repo.insert!(%Ango.SomeSchema{})
#
# We recommend using the bang functions (`insert!`, `update!`
# and so on) as they will fail if something goes wrong.

alias Ango.Repo
alias Ango.Schemas.{Batch, Code}

# Insert test batches
batch_1 = %{
  id: "11111111-1111-1111-1111-111111111111",
  name: "Summer Sale",
  rules: %{"maxpercustomer" => 1, "timelimit" => 30},
  expired: false
}

batch_2 = %{
  id: "22222222-2222-2222-2222-222222222222", 
  name: "Winter Promotion",
  rules: %{"maxpercustomer" => 2, "timelimit" => 30},
  expired: false
}

batch_3 = %{
  id: "33333333-3333-3333-3333-333333333333",
  name: "Exhausted Batch",
  rules: nil,
  expired: false
}

batch_4 = %{
  id: "44444444-4444-4444-4444-444444444444",
  name: "Expired Batch", 
  rules: nil,
  expired: true
}

# Insert batches
IO.puts("Creating batches...")

[batch_1, batch_2, batch_3, batch_4]
|> Enum.each(fn batch_attrs ->
  %Batch{}
  |> Batch.changeset(batch_attrs)
  |> Repo.insert!(on_conflict: :nothing)
end)

# Generate codes for batches 1 and 2
IO.puts("Creating codes for Summer Sale batch...")

for i <- 1..100 do
  code_value = "SUMMER2024-#{String.pad_leading("#{i}", 6, "0")}"
  
  %Code{}
  |> Code.changeset(%{
    code: code_value,
    batch_id: "11111111-1111-1111-1111-111111111111",
    client_id: "217be7c8-679c-4e08-bffc-db3451bdcdbf"
  })
  |> Repo.insert!(on_conflict: :nothing)
end

IO.puts("Creating codes for Winter Promotion batch...")

for i <- 1..100 do
  code_value = "WINTER2024-#{String.pad_leading("#{i}", 6, "0")}"
  
  %Code{}
  |> Code.changeset(%{
    code: code_value,
    batch_id: "22222222-2222-2222-2222-222222222222",
    client_id: "2ee73a08-ac6f-457d-934f-dcbc61840ae6"
  })
  |> Repo.insert!(on_conflict: :nothing)
end

# Create an exhausted code for batch 3
IO.puts("Creating exhausted code...")

%Code{}
|> Code.changeset(%{
  code: "EXHAUSTED-001",
  batch_id: "33333333-3333-3333-3333-333333333333", 
  client_id: "2ee73a08-ac6f-457d-934f-dcbc61840ae6",
  customer_id: UUID.uuid4(),
  used_at: DateTime.utc_now()
})
|> Repo.insert!(on_conflict: :nothing)

IO.puts("Database seeded successfully!")
IO.puts("- 4 batches created")
IO.puts("- 200 active codes created")
IO.puts("- 1 exhausted code created")
IO.puts("Ready for testing!")