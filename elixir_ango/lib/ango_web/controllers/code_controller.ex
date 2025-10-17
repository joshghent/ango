defmodule AngoWeb.CodeController do
  use AngoWeb, :controller
  
  require Logger
  alias Ango.Codes

  @doc """
  Redeem a code for a customer.
  
  Expected JSON payload:
  {
    "batchid": "uuid",
    "clientid": "uuid", 
    "customerid": "uuid"
  }
  """
  def redeem(conn, %{"batchid" => batch_id, "clientid" => client_id, "customerid" => customer_id}) do
    start_time = System.monotonic_time()

    case Codes.get_code(batch_id, client_id, customer_id) do
      {:ok, code} ->
        json(conn, %{code: code})

      {:error, :invalid_batch_id_format} ->
        conn
        |> put_status(:bad_request)
        |> json(%{error: "invalid batch_id format"})

      {:error, :invalid_client_id_format} ->
        conn
        |> put_status(:bad_request)
        |> json(%{error: "invalid client_id format"})

      {:error, :invalid_customer_id_format} ->
        conn
        |> put_status(:bad_request)
        |> json(%{error: "invalid customer_id format"})

      {:error, :no_batch_found} ->
        conn
        |> put_status(:not_found)
        |> json(%{error: "no batch found"})

      {:error, :batch_expired} ->
        conn
        |> put_status(:forbidden)
        |> json(%{error: "batch is expired"})

      {:error, :no_code_found} ->
        conn
        |> put_status(:not_found)
        |> json(%{error: "no code found"})

      {:error, :condition_not_met} ->
        conn
        |> put_status(:forbidden)
        |> json(%{error: "rule conditions not met"})

      {:error, :circuit_breaker_open} ->
        conn
        |> put_status(:service_unavailable)
        |> json(%{
          error: "Service temporarily unavailable",
          message: "Please try again later",
          retry_after: 30
        })

      {:error, reason} ->
        Logger.error("Code redemption failed: #{inspect(reason)}")
        conn
        |> put_status(:internal_server_error)
        |> json(%{error: "database error"})
    end
  end

  def redeem(conn, _params) do
    conn
    |> put_status(:bad_request)
    |> json(%{error: "cannot parse json"})
  end

  @doc """
  Upload codes via CSV file.
  
  Expected form data:
  - file: CSV file with 'client_id,code' columns
  - batch_name: Name for the batch
  - rules: Optional JSON rules
  """
  def upload(conn, %{"file" => file, "batch_name" => batch_name} = params) do
    rules = Map.get(params, "rules")

    with :ok <- validate_csv_file(file),
         {:ok, batch} <- create_batch_with_rules(batch_name, rules),
         :ok <- process_csv_upload(file, batch.id) do
      
      json(conn, %{message: "Codes uploaded successfully"})
    else
      {:error, :invalid_file_type} ->
        conn
        |> put_status(:bad_request)
        |> json(%{error: "File must be a CSV"})

      {:error, :missing_columns} ->
        conn
        |> put_status(:bad_request)
        |> json(%{error: "CSV must contain 'client_id' and 'code' columns"})

      {:error, reason} ->
        Logger.error("Code upload failed: #{inspect(reason)}")
        conn
        |> put_status(:internal_server_error)
        |> json(%{error: "Failed to upload codes: #{inspect(reason)}"})
    end
  end

  def upload(conn, %{"batch_name" => _}) do
    conn
    |> put_status(:bad_request)
    |> json(%{error: "No CSV file provided"})
  end

  def upload(conn, _params) do
    conn
    |> put_status(:bad_request)
    |> json(%{error: "Batch name is required"})
  end

  # Private functions

  defp validate_csv_file(%Plug.Upload{filename: filename}) do
    if String.ends_with?(filename, ".csv") do
      :ok
    else
      {:error, :invalid_file_type}
    end
  end

  defp create_batch_with_rules(batch_name, rules_json) when is_binary(rules_json) do
    case Jason.decode(rules_json) do
      {:ok, rules} ->
        Ango.Batches.create_batch(%{name: batch_name, rules: rules})
      
      {:error, _} ->
        {:error, :invalid_rules_json}
    end
  end

  defp create_batch_with_rules(batch_name, nil) do
    Ango.Batches.create_batch(%{name: batch_name, rules: %{}})
  end

  defp process_csv_upload(%Plug.Upload{path: path}, batch_id) do
    # Simple CSV processing without external library
    case File.read(path) do
      {:ok, content} ->
        lines = String.split(content, "\n", trim: true)
        
        case lines do
          [header | rows] ->
            if validate_csv_header(header) do
              csv_data = [String.split(header, ",") | Enum.map(rows, &String.split(&1, ","))]
              Codes.upload_codes(csv_data, batch_id)
              :ok
            else
              {:error, :missing_columns}
            end
          
          [] ->
            {:error, :empty_file}
        end
      
      {:error, reason} ->
        {:error, reason}
    end
  end

  defp validate_csv_header(header) do
    columns = String.split(header, ",")
    contains_required_columns?(columns, ["client_id", "code"])
  end



  defp contains_required_columns?(headers, required) do
    header_set = MapSet.new(headers)
    required_set = MapSet.new(required)
    MapSet.subset?(required_set, header_set)
  end
end