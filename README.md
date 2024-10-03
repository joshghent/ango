# **Ango (安居)**

Ango is a lightweight code distribution service built on Golang and Postgres. It's fully customisable and extensible to your needs.

## The Numbers
Ango has been load tested to death. That's what it's designed to do - handle huge volumes of load.
The setup was two digitalocean VPS's running k8s with a load balancer in front.
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
Codes is the primary place where we fetch and distribute codes from.
Codes can have certain rules associated with them (see below) and are associated with a particular batch and client.

### Batches
Often times with codes they are grouped into batches. For example, an ecommerce business may have a "Summer sale" and discount codes associated with that.
Batches are designed so that you can easily remove/expiry discount codes without having to know what each discount code is.
Batches are always associated with clients and can have one or more codes.

### Clients
Clients are your clients in your system. For example, if you are a ticketing business, you want to denote what codes are associated with which band that is performing - this would be marked with the client, with the performance being the "batch".

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

### To download

### To test

### Integrating in your app

### Importing Codes via CSV

## Why is it called Ango?
Ango is a 3 month period of intensive study for zen buddists. I'm not one myself, but this project was a learning exercise for me to get better at golang. And it taught me a great deal about parallell computation.

## License
