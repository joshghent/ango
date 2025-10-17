defmodule AngoWeb.CodeControllerTest do
  use AngoWeb.ConnCase, async: true

  alias Ango.{Repo, Schemas.Code, Schemas.Batch}

  describe "POST /api/v1/code/redeem" do
    setup do
      # Create test batch with codes
      batch = %Batch{
        id: "11111111-1111-1111-1111-111111111111",
        name: "Test Batch",
        rules: %{"maxpercustomer" => 1, "timelimit" => 30},
        expired: false
      } |> Repo.insert!()

      client_id = "217be7c8-679c-4e08-bffc-db3451bdcdbf"
      
      code = %Code{
        code: "TEST-CODE-123",
        batch_id: batch.id,
        client_id: client_id,
        customer_id: nil,
        used_at: nil
      } |> Repo.insert!()

      {:ok, batch: batch, code: code, client_id: client_id}
    end

    test "successfully redeems a code", %{conn: conn, batch: batch, client_id: client_id} do
      customer_id = UUID.uuid4()
      
      payload = %{
        "batchid" => batch.id,
        "clientid" => client_id,
        "customerid" => customer_id
      }

      conn = post(conn, "/api/v1/code/redeem", payload)
      
      assert %{"code" => code_value} = json_response(conn, 200)
      assert is_binary(code_value)
      assert String.length(code_value) > 0
    end

    test "returns 400 for invalid batch_id format", %{conn: conn, client_id: client_id} do
      payload = %{
        "batchid" => "invalid-uuid",
        "clientid" => client_id,
        "customerid" => UUID.uuid4()
      }

      conn = post(conn, "/api/v1/code/redeem", payload)
      
      assert %{"error" => "invalid batch_id format"} = json_response(conn, 400)
    end

    test "returns 400 for invalid client_id format", %{conn: conn, batch: batch} do
      payload = %{
        "batchid" => batch.id,
        "clientid" => "invalid-uuid",
        "customerid" => UUID.uuid4()
      }

      conn = post(conn, "/api/v1/code/redeem", payload)
      
      assert %{"error" => "invalid client_id format"} = json_response(conn, 400)
    end

    test "returns 400 for invalid customer_id format", %{conn: conn, batch: batch, client_id: client_id} do
      payload = %{
        "batchid" => batch.id,
        "clientid" => client_id,
        "customerid" => "invalid-uuid"
      }

      conn = post(conn, "/api/v1/code/redeem", payload)
      
      assert %{"error" => "invalid customer_id format"} = json_response(conn, 400)
    end

    test "returns 404 for non-existent batch", %{conn: conn, client_id: client_id} do
      payload = %{
        "batchid" => UUID.uuid4(),
        "clientid" => client_id,
        "customerid" => UUID.uuid4()
      }

      conn = post(conn, "/api/v1/code/redeem", payload)
      
      assert %{"error" => "no batch found"} = json_response(conn, 404)
    end

    test "returns 404 when no codes available", %{conn: conn, batch: batch, code: code, client_id: client_id} do
      # Mark the only code as used
      code
      |> Code.changeset(%{
        customer_id: UUID.uuid4(),
        used_at: DateTime.utc_now()
      })
      |> Repo.update!()

      payload = %{
        "batchid" => batch.id,
        "clientid" => client_id,
        "customerid" => UUID.uuid4()
      }

      conn = post(conn, "/api/v1/code/redeem", payload)
      
      assert %{"error" => "no code found"} = json_response(conn, 404)
    end

    test "returns 403 when customer exceeds limits", %{conn: conn, batch: batch, client_id: client_id} do
      customer_id = UUID.uuid4()
      
      # First redemption
      payload = %{
        "batchid" => batch.id,
        "clientid" => client_id,
        "customerid" => customer_id
      }

      conn1 = post(conn, "/api/v1/code/redeem", payload)
      assert json_response(conn1, 200)

      # Create another code for second attempt
      %Code{
        code: "TEST-CODE-456",
        batch_id: batch.id,
        client_id: client_id,
        customer_id: nil,
        used_at: nil
      } |> Repo.insert!()

      # Second redemption should fail (limit is 1)
      conn2 = post(conn, "/api/v1/code/redeem", payload)
      assert %{"error" => "rule conditions not met"} = json_response(conn2, 403)
    end

    test "returns 400 for malformed JSON", %{conn: conn} do
      conn = post(conn, "/api/v1/code/redeem", %{})
      
      assert %{"error" => "cannot parse json"} = json_response(conn, 400)
    end
  end
end