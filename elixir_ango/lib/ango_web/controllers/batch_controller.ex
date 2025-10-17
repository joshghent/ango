defmodule AngoWeb.BatchController do
  use AngoWeb, :controller
  
  alias Ango.Batches

  @doc """
  List all active batches.
  """
  def index(conn, _params) do
    batches = 
      Batches.list_batches()
      |> Enum.map(&format_batch/1)
    
    json(conn, batches)
  end

  # Private functions

  defp format_batch(batch) do
    %{
      id: batch.id,
      name: batch.name,
      rules: batch.rules || %{},
      expired: batch.expired
    }
  end
end