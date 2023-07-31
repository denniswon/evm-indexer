<!-- omit in toc -->

# ValidationCloud

ValidationCloud is a service which uses the Ethereum JSON-RPC API to store the following information in a local datastore (postgres) for the most recent 50 blocks (or more) and provide REST APIs:

- Get and store **the block** and **all transaction hashes** in the block
- Get and store all **events related to each transaction in each block**
- Expose an endpoint that allows a user to **query all events related to a particular address**

The services provided:

- Historical data query

<!-- omit in toc -->

## Overview

- Sync up to latest state of blockchain
- Listen for all happenings on Ethereum or EVM based blockchain
- Persist all happenings in local database
- Expose REST API for querying, while also setting block range/ time range for filtering results. Allow querying latest **X** entries for events emitted by contracts.
  - Block data
  - Transaction data
  - Event data

<!-- omit in toc -->

## Table of Contents

- [ValidationCloud](#validationcloud)
  - [Overview](#overview)
  - [Table of Contents](#table-of-contents)
  - [Prerequisite](#prerequisite)
  - [Installation](#installation)
    - [.env configuration](#env-configuration)
  - [Usage](#usage)
    - [Historical Block Data ( REST API )](#historical-block-data--rest-api-)
    - [Historical Transaction Data ( REST API )](#historical-transaction-data--rest-api-)
    - [Historical Event Data ( REST API )](#historical-event-data--rest-api-)
  - [Notes:](#notes)
    - [Thought process and code design](#thought-process-and-code-design)
    - [Technology choices](#technology-choices)
    - [How would you keep the data set up to date?](#how-would-you-keep-the-data-set-up-to-date)
    - [How would you expose the stored data to customers in an easy-to-query API?](#how-would-you-expose-the-stored-data-to-customers-in-an-easy-to-query-api)
    - [How would you handle security of the API?](#how-would-you-handle-security-of-the-api)
    - [How would you improve the performance of your approach?](#how-would-you-improve-the-performance-of-your-approach)
    - [How would you adapt your design to store the same data for the entire history of Ethereum Mainnet?](#how-would-you-adapt-your-design-to-store-the-same-data-for-the-entire-history-of-ethereum-mainnet)
    - [What would it take to deploy and monitor a service like this in production?](#what-would-it-take-to-deploy-and-monitor-a-service-like-this-in-production)

## Prerequisite

- Go _( >= 1.15 )_
- Both **HTTP & Websocket** RPC connection endpoint required

  > Querying block, transaction, event log related data using HTTP

- Install & set up PostgreSQL. [guide](https://www.digitalocean.com/community/tutorials/how-to-install-and-use-postgresql-on-ubuntu-20-04).

  > Enable `pgcrypto` extension on PostgreSQL Database.

  > Create extension: `create extension pgcrypto;`

  > Check extension: `\dx`

  > Make sure PostgreSQL has md5 authentication mechanism enabled.

## Installation

```bash
git clone git@github.com:denniswon/validationcloud.git

cd validationcloud

cp .env.example .env
```

### .env configuration

- For processing block(s)/ tx(s) concurrently, it'll create `ConcurrencyFactor * #-of CPUs on machine` workers, who will pick up jobs submitted to them.

  - If nothing is specified, it defaults to 1 & assuming you're running on machine with 4 CPUs, it'll spawn worker pool of size 4. More than configured number of jobs can be submitted, only 4 can be running at max.

- For range based queries `BlockRange` can be set to limit how many blocks can be queried by client in a single go. Default value 100.

- For time span based queries `TimeRange` can be set to put limit on max time span _( in terms of second )_, can be used by clients. Default value 3600 i.e. 1 hour.

```
RPCUrl=https://<rpc-endpoint>
WebsocketUrl=wss://<websocket-endpoint>

PORT=7000

DB_USER=user
DB_PASSWORD=password
DB_HOST=x.x.x.x
DB_PORT=5432
DB_NAME=validationcloud

Production=yes

ConcurrencyFactor=5
BlockRange=1000
TimeRange=21600
```

- Build `validationcloud`

```bash
go mod tidy

make build
```

- Run `validationcloud`

```bash
./validationcloud

# or to build & run together
make run
```

- Database migration taken care of during application start up.

- Syncing with latest state of blockchain takes time. Current sync state can be queried

```bash
curl -s localhost:7000/v1/synced | jq
```

```json
{
  "elapsed": "3m2.487237s",
  "eta": "87h51m38s",
  "processed": 4242,
  "synced": "0.35 %"
}
```

## Usage

`validationcloud` exposes REST API for querying historical block, transaction & event related data. It can also play role of real time notification engine, when subscribed to supported topics.

### Historical Block Data ( REST API )

You can query historical block data with various combination of query string params.

**Path : `/v1/block`**

| Query Params                            | Method | Description                                                           |
| --------------------------------------- | ------ | --------------------------------------------------------------------- |
| `hash=0x...&tx=yes`                     | GET    | Fetch all transactions present in a block, when block hash is known   |
| `number=1&tx=yes`                       | GET    | Fetch all transactions present in a block, when block number is known |
| `hash=0x...`                            | GET    | Fetch block by hash                                                   |
| `number=1`                              | GET    | Fetch block by number                                                 |
| `fromBlock=1&toBlock=10`                | GET    | Fetch blocks by block number range _( max 10 at a time )_             |
| `fromTime=1604975929&toTime=1604975988` | GET    | Fetch blocks by unix timestamp range _( max 60 seconds timespan )_    |

### Historical Transaction Data ( REST API )

**Path : `/v1/transaction`**

| Query Params                                                              | Method | Description                                                                                                                                                          |
| ------------------------------------------------------------------------- | ------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `hash=0x...`                                                              | GET    | Fetch transaction by txHash                                                                                                                                          |
| `nonce=1&fromAccount=0x...`                                               | GET    | Fetch transaction, when tx sender's address & account nonce are known                                                                                                |
| `fromBlock=1&toBlock=10&deployer=0x...`                                   | GET    | Find out what contracts are created by certain account within given block number range _( max 100 blocks )_                                                          |
| `fromTime=1604975929&toTime=1604975988&deployer=0x...`                    | GET    | Find out what contracts are created by certain account within given timestamp range _( max 600 seconds of timespan )_                                                |
| `fromBlock=1&toBlock=100&fromAccount=0x...&toAccount=0x...`               | GET    | Given block number range _( max 100 at a time )_ & a pair of accounts, can find out all tx performed between that pair, where `from` & `to` fields are fixed         |
| `fromTime=1604975929&toTime=1604975988&fromAccount=0x...&toAccount=0x...` | GET    | Given time stamp range _( max 600 seconds of timespan )_ & a pair of accounts, can find out all tx performed between that pair, where `from` & `to` fields are fixed |
| `fromBlock=1&toBlock=100&fromAccount=0x...`                               | GET    | Given block number range _( max 100 at a time )_ & an account, can find out all tx performed from that account                                                       |
| `fromTime=1604975929&toTime=1604975988&fromAccount=0x...`                 | GET    | Given time stamp range _( max 600 seconds of span )_ & an account, can find out all tx performed from that account                                                   |
| `fromBlock=1&toBlock=100&toAccount=0x...`                                 | GET    | Given block number range _( max 100 at a time )_ & an account, can find out all tx where target was this address                                                     |
| `fromTime=1604975929&toTime=1604975988&toAccount=0x...`                   | GET    | Given time stamp range _( max 600 seconds of span )_ & an account, can find out all tx where target was this address                                                 |

### Historical Event Data ( REST API )

**Path : `/v1/event`**

| Query Params                                                                                               | Method | Description                                                                                                          |
| ---------------------------------------------------------------------------------------------------------- | ------ | -------------------------------------------------------------------------------------------------------------------- |
| `blockHash=0x...`                                                                                          | GET    | Given blockhash, retrieves all events emitted by tx(s) present in block                                              |
| `blockHash=0x...&logIndex=1`                                                                               | GET    | Given blockhash and log index in block, attempts to retrieve associated event                                        |
| `blockNumber=123456&logIndex=2`                                                                            | GET    | Given block number and log index in block, attempts to retrieve associated event                                     |
| `txHash=0x...`                                                                                             | GET    | Given txhash, retrieves all events emitted during execution of this transaction                                      |
| `count=50&contract=0x...`                                                                                  | GET    | Returns last **x** _( <=50 )_ events emitted by this contract                                                        |
| `fromBlock=1&toBlock=10&contract=0x...&topic0=0x...&topic1=0x...&topic2=0x...&topic3=0x...`                | GET    | Finding event(s) emitted from contract within given block range & also matching topic signatures _{0, 1, 2, 3}_      |
| `fromBlock=1&toBlock=10&contract=0x...&topic0=0x...&topic1=0x...&topic2=0x...`                             | GET    | Finding event(s) emitted from contract within given block range & also matching topic signatures _{0, 1, 2}_         |
| `fromBlock=1&toBlock=10&contract=0x...&topic0=0x...&topic1=0x...`                                          | GET    | Finding event(s) emitted from contract within given block range & also matching topic signatures _{0, 1}_            |
| `fromBlock=1&toBlock=10&contract=0x...&topic0=0x...`                                                       | GET    | Finding event(s) emitted from contract within given block range & also matching topic signatures _{0}_               |
| `fromBlock=1&toBlock=10&contract=0x...`                                                                    | GET    | Finding event(s) emitted from contract within given block range                                                      |
| `fromTime=1604975929&toTime=1604975988&contract=0x...&topic0=0x...&topic1=0x...&topic2=0x...&topic3=0x...` | GET    | Finding event(s) emitted from contract within given time stamp range & also matching topic signatures _{0, 1, 2, 3}_ |
| `fromTime=1604975929&toTime=1604975988&contract=0x...&topic0=0x...&topic1=0x...&topic2=0x...`              | GET    | Finding event(s) emitted from contract within given time stamp range & also matching topic signatures _{0, 1, 2}_    |
| `fromTime=1604975929&toTime=1604975988&contract=0x...&topic0=0x...&topic1=0x...`                           | GET    | Finding event(s) emitted from contract within given time stamp range & also matching topic signatures _{0, 1}_       |
| `fromTime=1604975929&toTime=1604975988&contract=0x...&topic0=0x...`                                        | GET    | Finding event(s) emitted from contract within given time stamp range & also matching topic signatures _{0}_          |
| `fromTime=1604975929&toTime=1604975988&contract=0x...`                                                     | GET    | Finding event(s) emitted from contract within given time stamp range                                                 |

<!-- omit in toc -->

## Notes:

- Code is written in Go

<!-- omit in toc -->

### Thought process and code design

- Concurrency support using event request queue

<!-- omit in toc -->

### Technology choices

- Postgres for persistant local storage
- gin for servering api
- go-ethereum ethclient for blockchain communication

<!-- omit in toc -->

### How would you keep the data set up to date?

- ethclient websocket listening for new blocks

<!-- omit in toc -->

### How would you expose the stored data to customers in an easy-to-query API?

- Rest API / GraphQL

<!-- omit in toc -->

### How would you handle security of the API?

- postgres authentication
- API key for users
- API delivery stats and history

<!-- omit in toc -->

### How would you improve the performance of your approach?

- First vertical scaling
- Then horizontal scaling with transition to more distributed architecture
- DB sharding by blocks / tx and event assoicated address indexing
- Load balancer
- API key and user session management with in-memory caching of api requests and responses
- All historical data query requests to require ApiKey
- Subscription plan with tiers

<!-- omit in toc -->

### How would you adapt your design to store the same data for the entire history of Ethereum Mainnet?

- Snapshotting for scalability, add support for features to take snapshots of the db and restore from snapshots
- Snapshotting also useful for migrating to different machine or setting up new instance,to avoid a lengthy whole chain data syncing.

- DB sharding by block number ranges and indexing
- tx and event assoicated address sharding and indexing

<!-- omit in toc -->

### What would it take to deploy and monitor a service like this in production?

- Kubernetes, Terraform and Ansible
- Integration and monitoring tests
- Grafana, Amplitude, Sentry, Pagerduty
- RPC endpoint fallbacks
- Production deployment using **systemd**
