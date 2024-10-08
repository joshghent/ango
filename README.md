# **Ango (暗号)**

Ango - **means code 🧑‍💻 in Japanese** - is a lightweight code distribution service built on Golang and Postgres. It's fully customisable and extensible to your needs.

## The Numbers
Ango has been load tested to death. That's what it's designed to do - handle huge volumes of load.
The setup was
Results
```

```

## Use cases
Ango can be used for anything where you want a customer to make a request and recieve a unique code back (with no duplicates).
For example:
* Discount code distribution
* Gift card codes
* Referral codes
* One time passwords
* Ticket codes

## Concepts
Ango was designed to be flexible but has a reasonable opionated setup out of the box for easy install.
Below are the key concepts to familiarise yourself with.

### Codes
Codes are the primary place where we fetch and distribute codes from.
Codes can have certain rules associated with them (see below) and are associated with a particular batch and client.

### Batches
Often times with codes they are grouped into batches. For example, an ecommerce business may have a "Summer sale" and discount codes associated with that.
Batches are designed so that you can easily remove/expiry discount codes without having to know what each discount code is.
Batches are always associated with clients and can have one or more codes.

### Clients
Clients are **your** clients in **your** system. For example, if you are a ticketing business, you want to denote what codes are associated with which band that is performing - this would be marked with the client, with the performance being the "batch".

### Rules
Batches can have rules. These are super extensible, thanks to being JSON based.
Out of the box we have functionality to limit codes to N per customer and within a time limit. For example, 2 per customer every month.
This can be expanded but you will need to update the code in `checkRules`.

#### Sample Batch Record

Here's an example of a batch record with associated rules:
```
{
  "id": "11111111-1111-1111-1111-111111111111",
  "name": "Summer Sale",
  "rules": {
    "maxpercustomer": 1,
    "timelimit": 30 // days - optional, defaults to unlimited
  },
  "expired": false
}
```


## Install / Setup
Locally, you can run Ango with docker compose:
```
docker compose up -d

# or in production
docker compose -f docker-compose.prod.yml up -d
```

Then you can run the migrations and seed the database:
```
make migrate
make seed
```

### To test
```
make test
```

### To create a migration
We use the db-migrate tool to manage migrations.
```
migrate create -ext sql -dir db/migrations -seq <name>
```


### Integrating in your app
Ango is designed to be whitelabel and unopionated. Here are some things you need to consider when integrating:
* You will need to perform authentication prior to calling Ango's API.
* Rate limiting is not included but can be added by you.
* Integration can be done by simply spinning up Ango and using the API.

### Redeeming codes
```shell
curl --request POST \
  --url http://your-ango-server/api/v1/code/redeem \
  --header 'content-type: application/json' \
  --data '{
  "batchid": "11111111-1111-1111-1111-111111111111",
  "clientid": "217be7c8-679c-4e08-bffc-db3451bdcdbf",
  "customerid": "50b0b41b-c665-4409-a2bb-a4fc18828dc2"
}'

# {
#  "code": "73619c34-e941-4384-bb98-3a2ff094ddd0"
# }
```

### Fetching batches
```shell
curl --request GET \
  --url http://localhost:3000/api/v1/batches

# [
#   {
#     "id": "11111111-1111-1111-1111-111111111111",
#     "name": "Winter Batch",
#     "rules": {
#       "maxpercustomer": 1,
#       "timelimit": 30 // days
#     },
#     "expired": false
#   },
#   {
#     "id": "22222222-2222-2222-2222-222222222222",
#     "name": "Summer Sale",
#     "rules": {
#       "maxpercustomer": 5,
#       "timelimit": 90 // days
#     },
#     "expired": false
#   }
# ]
```

### Importing Codes via CSV

You can import codes into Ango using a CSV file through the `/api/v1/codes/upload` endpoint. Here's how to use it:

1. Prepare your CSV file:
   - The CSV should have two columns: `client_id`, and `code`.
   - The first row should be the header row with these column names.
   - Each subsequent row should contain the data for one code.

2. Make a POST request to `/api/v1/codes/upload`:
   - Use multipart/form-data as the content type.
   - Include the following form fields:
     - `file`: Your CSV file
     - `batch_name`: The name of the batch you're creating
     - `rules` (optional): A JSON string containing the rules for this batch

3. Example using curl:
   ```
   curl -X POST http://your-ango-server/api/v1/codes/upload \
     -F "file=@/path/to/your/codes.csv" \
     -F "batch_name=Summer Sale 2023" \
     -F 'rules={"maxpercustomer":2,"timelimit":30}'
   ```

4. The server will respond with a success message if the upload is successful, or an error message if there's a problem.

Note: Ensure that your CSV file is properly formatted and that the client_ids in the CSV file exist in your system.

## License

This project is licensed under the MIT License. This license allows businesses to use, modify, and distribute the software, provided they include the original copyright notice and disclaimer. The full text of the MIT License can be found at: https://opensource.org/licenses/MIT

When using this software, please include the following attribution:
"This product includes software developed by [Josh Ghent/Turbo Technologies] (https://github.com/joshghent/ango)."
