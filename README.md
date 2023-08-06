<!-- omit in toc -->
# ValidationCloud

ValidationCloud is a service which uses the Ethereum JSON-RPC API to store the following information in a local datastore (postgres) for the most recent 50 blocks (or more) and provide REST and Graphql APIs:

- Get and store **the block** and **all transaction hashes** in the block
- Get and store all **events related to each transaction in each block**
- Expose an endpoint that allows a user to **query all events related to a particular address**

The services provided:
- Historical data query
- Real-time notification support

<!-- omit in toc -->
## Overview
- Sync up to latest state of blockchain
- Listen for all happenings on Ethereum or EVM based blockchain
- Persist all happenings in local database
- Expose REST & GraphQL API for querying, while also setting block range/ time range for filtering results. Allow querying latest **X** entries for events emitted by contracts.
  - Block data
  - Transaction data
  - Event data
- Expose websocket based real time notification mechanism for
  - Blocks being mined
  - Transactions being sent from address and/ or transactions being received at address
  - Events being emitted by contract, with indexed fields i.e. topics
- It has capability to process blocks in delayed fashion, if configured to do so. **To address chain reorganization issue**. Configuration specifies how many block confirmations you require before considering that block to be finalized in `.env` file.
- If real-time subscription mode is enabled, it'll publish data to clients who're interested i.e. subscribed.
- For delayed mode, blocks are processed the same way, except putting it in persistent data store. Rather block identifier to be put in **waiting queue**, from where it'll be eventually picked up by workers to finally persist it in DB.
- Downside of using this feature is you might not get data back in response of query for certain block number, which just got mined but not finalized as per configured i.e. `BlockConfirmations` environment variable's value. Default value will be **0**.

<!-- omit in toc -->
## Table of Contents
- [Prerequisite](#prerequisite)
- [Installation](#installation)
  - [.env configuration](#env-configuration)
- [Usage](#usage)
  - [Historical Block Data ( REST API )](#historical-block-data--rest-api-)
  - [Historical Transaction Data ( REST API )](#historical-transaction-data--rest-api-)
  - [Historical Event Data ( REST API )](#historical-event-data--rest-api-)
  - [Historical Block Data ( GraphQL API )](#historical-block-data--graphql-api-)
  - [Historical Transaction Data ( GraphQL API )](#historical-transaction-data--graphql-api-)
  - [Historical Event Data ( GraphQL API )](#historical-event-data--graphql-api-)
  - [Real time notification for mined blocks](#real-time-notification-for-mined-blocks)
  - [Real time notification for transactions](#real-time-notification-for-transactions)
  - [Real-time notification for events](#real-time-notification-for-events)
## Prerequisite
- Go _( >= 1.15 )_
- Both **HTTP & Websocket** RPC connection endpoint required
  > Querying block, transaction, event log related data using HTTP

  > Listening for block mining events in real time over Websocket
- Install & set up PostgreSQL. [guide](https://www.digitalocean.com/community/tutorials/how-to-install-and-use-postgresql-on-ubuntu-20-04).
    > Enable `pgcrypto` extension on PostgreSQL Database.

    > Create extension: `create extension pgcrypto;`

    > Check extension: `\dx`

    > Make sure PostgreSQL has md5 authentication mechanism enabled.

- Install and set up Redis [guide](https://www.digitalocean.com/community/tutorials/how-to-install-and-secure-redis-on-ubuntu-20-04).
  - Note : Redis **v6.0.6** is required

## Installation

```bash
git clone git@github.com:denniswon/validationcloud.git

cd validationcloud

cp .env.example .env
```

### .env configuration

- For testing historical data query using browser based GraphQL Playground, set `GraphQLPlayGround` to `yes` in config file

- For processing block(s)/ tx(s) concurrently, it'll create `ConcurrencyFactor * #-of CPUs on machine` workers, who will pick up jobs submitted to them.
  - If nothing is specified, it defaults to 1 & assuming you're running on machine with 4 CPUs, it'll spawn worker pool of size 4. More than configured number of jobs can be submitted, only 4 can be running at max.

- For delayed mode, set `BlockConfirmations` to some _number > 0_.

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

RedisConnection=tcp
RedisAddress=x.x.x.x:6379
RedisPassword=password

Production=yes

ConcurrencyFactor=5
BlockConfirmations=200
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

`validationcloud` exposes REST & GraphQL API for querying historical block, transaction & event related data. It can also play role of real time notification engine, when subscribed to supported topics.

### Historical Block Data ( REST API )

You can query historical block data with various combination of query string params.

**Path : `/v1/block`**

Query Params | Method | Description
--- | --- | ---
`hash=0x...&tx=yes` | GET | Fetch all transactions present in a block, when block hash is known
`number=1&tx=yes` | GET | Fetch all transactions present in a block, when block number is known
`hash=0x...` | GET | Fetch block by hash
`number=1` | GET | Fetch block by number
`fromBlock=1&toBlock=10` | GET | Fetch blocks by block number range _( max 10 at a time )_
`fromTime=1604975929&toTime=1604975988` | GET | Fetch blocks by unix timestamp range _( max 60 seconds timespan )_

### Historical Transaction Data ( REST API )

**Path : `/v1/transaction`**

Query Params | Method | Description
--- | --- | ---
`hash=0x...` | GET | Fetch transaction by txHash
`nonce=1&fromAccount=0x...` | GET | Fetch transaction, when tx sender's address & account nonce are known
`fromBlock=1&toBlock=10&deployer=0x...` | GET | Find out what contracts are created by certain account within given block number range _( max 100 blocks )_
`fromTime=1604975929&toTime=1604975988&deployer=0x...` | GET | Find out what contracts are created by certain account within given timestamp range _( max 600 seconds of timespan )_
`fromBlock=1&toBlock=100&fromAccount=0x...&toAccount=0x...` | GET | Given block number range _( max 100 at a time )_ & a pair of accounts, can find out all tx performed between that pair, where `from` & `to` fields are fixed
`fromTime=1604975929&toTime=1604975988&fromAccount=0x...&toAccount=0x...` | GET | Given time stamp range _( max 600 seconds of timespan )_ & a pair of accounts, can find out all tx performed between that pair, where `from` & `to` fields are fixed
`fromBlock=1&toBlock=100&fromAccount=0x...` | GET | Given block number range _( max 100 at a time )_ & an account, can find out all tx performed from that account
`fromTime=1604975929&toTime=1604975988&fromAccount=0x...` | GET | Given time stamp range _( max 600 seconds of span )_ & an account, can find out all tx performed from that account
`fromBlock=1&toBlock=100&toAccount=0x...` | GET | Given block number range _( max 100 at a time )_ & an account, can find out all tx where target was this address
`fromTime=1604975929&toTime=1604975988&toAccount=0x...` | GET | Given time stamp range _( max 600 seconds of span )_ & an account, can find out all tx where target was this address

### Historical Event Data ( REST API )

**Path : `/v1/event`**

Query Params | Method | Description
--- | --- | ---
`blockHash=0x...` | GET | Given blockhash, retrieves all events emitted by tx(s) present in block
`blockHash=0x...&logIndex=1` | GET | Given blockhash and log index in block, attempts to retrieve associated event
`blockNumber=123456&logIndex=2` | GET | Given block number and log index in block, attempts to retrieve associated event
`txHash=0x...` | GET | Given txhash, retrieves all events emitted during execution of this transaction
`count=50&contract=0x...` | GET | Returns last **x** _( <=50 )_ events emitted by this contract
`fromBlock=1&toBlock=10&contract=0x...&topic0=0x...&topic1=0x...&topic2=0x...&topic3=0x...` | GET | Finding event(s) emitted from contract within given block range & also matching topic signatures _{0, 1, 2, 3}_
`fromBlock=1&toBlock=10&contract=0x...&topic0=0x...&topic1=0x...&topic2=0x...` | GET | Finding event(s) emitted from contract within given block range & also matching topic signatures _{0, 1, 2}_
`fromBlock=1&toBlock=10&contract=0x...&topic0=0x...&topic1=0x...` | GET | Finding event(s) emitted from contract within given block range & also matching topic signatures _{0, 1}_
`fromBlock=1&toBlock=10&contract=0x...&topic0=0x...` | GET | Finding event(s) emitted from contract within given block range & also matching topic signatures _{0}_
`fromBlock=1&toBlock=10&contract=0x...` | GET | Finding event(s) emitted from contract within given block range
`fromTime=1604975929&toTime=1604975988&contract=0x...&topic0=0x...&topic1=0x...&topic2=0x...&topic3=0x...` | GET | Finding event(s) emitted from contract within given time stamp range & also matching topic signatures _{0, 1, 2, 3}_
`fromTime=1604975929&toTime=1604975988&contract=0x...&topic0=0x...&topic1=0x...&topic2=0x...` | GET | Finding event(s) emitted from contract within given time stamp range & also matching topic signatures _{0, 1, 2}_
`fromTime=1604975929&toTime=1604975988&contract=0x...&topic0=0x...&topic1=0x...` | GET | Finding event(s) emitted from contract within given time stamp range & also matching topic signatures _{0, 1}_
`fromTime=1604975929&toTime=1604975988&contract=0x...&topic0=0x...` | GET | Finding event(s) emitted from contract within given time stamp range & also matching topic signatures _{0}_
`fromTime=1604975929&toTime=1604975988&contract=0x...` | GET | Finding event(s) emitted from contract within given time stamp range

### Historical Block Data ( GraphQL API )

You can query block data using GraphQL API.

**Path: `/v1/graphql`**

**Method: `POST`**

```graphql
type Query {
    blockByHash(hash: String!): Block!
    blockByNumber(number: String!): Block!
    blocksByNumberRange(from: String!, to: String!): [Block!]!
    blocksByTimeRange(from: String!, to: String!): [Block!]!
}
```

Response:

```graphql
type Block {
  hash: String!
  number: String!
  time: String!
  parentHash: String!
  difficulty: String!
  gasUsed: String!
  gasLimit: String!
  nonce: String!
  miner: String!
  size: Float!
  txRootHash: String!
  receiptRootHash: String!
}
```

Method | Parameters | Possible use case
--- | --- | ---
`blockByHash` | hash: String! | When you know block hash & want to get whole block data back
`blockByNumber` | number: String! | When you know block number & want to get whole block data back
`blocksByNumberRange` | from: String!, to: String! | When you've a block number range & want to get all blocks in that range, in a single call
`blocksByTimeRange` | from: String!, to: String! | When you've unix timestamp range & want to get all blocks in that range, in a single call

---

### Historical Transaction Data ( GraphQL API )

**Path: `/v1/graphql`**

**Method: `POST`**

```graphql
type Query {
    transaction(hash: String!): Transaction!
  
    transactionCountByBlockHash(hash: String!): Int!
    transactionsByBlockHash(hash: String!): [Transaction!]!
  
    transactionCountByBlockNumber(number: String!): Int!
    transactionsByBlockNumber(number: String!): [Transaction!]!
  
    transactionCountFromAccountByNumberRange(account: String!, from: String!, to: String!): Int!
    transactionsFromAccountByNumberRange(account: String!, from: String!, to: String!): [Transaction!]!
  
    transactionCountFromAccountByTimeRange(account: String!, from: String!, to: String!): Int!
    transactionsFromAccountByTimeRange(account: String!, from: String!, to: String!): [Transaction!]!
  
    transactionCountToAccountByNumberRange(account: String!, from: String!, to: String!): Int!
    transactionsToAccountByNumberRange(account: String!, from: String!, to: String!): [Transaction!]!

    transactionCountToAccountByTimeRange(account: String!, from: String!, to: String!): Int!
    transactionsToAccountByTimeRange(account: String!, from: String!, to: String!): [Transaction!]!

    transactionCountBetweenAccountsByNumberRange(fromAccount: String!, toAccount: String!, from: String!, to: String!): Int!
    transactionsBetweenAccountsByNumberRange(fromAccount: String!, toAccount: String!, from: String!, to: String!): [Transaction!]!

    transactionCountBetweenAccountsByTimeRange(fromAccount: String!, toAccount: String!, from: String!, to: String!): Int!
    transactionsBetweenAccountsByTimeRange(fromAccount: String!, toAccount: String!, from: String!, to: String!): [Transaction!]!

    contractsCreatedFromAccountByNumberRange(account: String!, from: String!, to: String!): [Transaction!]!
    contractsCreatedFromAccountByTimeRange(account: String!, from: String!, to: String!): [Transaction!]!
    transactionFromAccountWithNonce(account: String!, nonce: String!): Transaction!
}
```

Response:

```graphql
type Transaction {
  hash: String!
  from: String!
  to: String!
  contract: String!
  value: String!
  data: String!
  gas: String!
  gasPrice: String!
  cost: String!
  nonce: String!
  state: String!
  blockHash: String!
}
```

Method | Parameters | Possible use case
--- | --- | ---
`transaction` | hash: String! | When you know txHash & want to get that tx data
`transactionCountByBlockHash` | hash: String! | When you know block hash & want to get count of tx(s) packed in that block
`transactionsByBlockHash` | hash: String! | When you know block hash & want to get all tx(s) packed in that block
`transactionCountByBlockNumber` | number: String! | When you know block number & want to get count of tx(s) packed in that block
`transactionsByBlockNumber` | number: String! | When you know block number & want to get all tx(s) packed in that block
`transactionCountFromAccountByNumberRange` | account: String!, from: String!, to: String! | When you know tx sender address, block number range & want to find out how many tx(s) were sent by this address in that certain block number range
`transactionsFromAccountByNumberRange` | account: String!, from: String!, to: String! | When you know tx sender address, block number range & want to find out all tx(s) that were sent by this address in that certain block number range
`transactionCountFromAccountByTimeRange` | account: String!, from: String!, to: String! | When you know tx sender address, unix time stamp range & want to find out how many tx(s) were sent by this address in that certain timespan
`transactionsFromAccountByTimeRange` | account: String!, from: String!, to: String! | When you know tx sender address, unix time stamp range & want to find out all tx(s) that were sent by this address in that certain timespan
`transactionCountToAccountByNumberRange` | account: String!, from: String!, to: String! | When you know tx receiver address, block number range & want to find out how many tx(s) were sent to this address in that certain block number range
`transactionsToAccountByNumberRange` | account: String!, from: String!, to: String! | When you know tx receiver address, block number range & want to find out all tx(s) that were sent to this address in that certain block number range
`transactionCountToAccountByTimeRange` | account: String!, from: String!, to: String! | When you know tx receiver address, unix time stamp range & want to find out how many tx(s) were sent to this address in that certain timespan
`transactionsToAccountByTimeRange` | account: String!, from: String!, to: String! | When you know tx receiver address, unix time stamp range & want to find out all tx(s) that were sent to this address in that certain timespan
`transactionCountBetweenAccountsByNumberRange` | fromAccount: String!, toAccount: String!, from: String!, to: String! | When you know tx sender & receiver addresses, block number range & want to find out how many tx(s) were sent from sender to receiver in that certain block number range
`transactionsBetweenAccountsByNumberRange` | fromAccount: String!, toAccount: String!, from: String!, to: String! | When you know tx sender & receiver addresses, block number range & want to find out all tx(s) that were sent from sender to receiver in that certain block number range
`transactionCountBetweenAccountsByTimeRange` | fromAccount: String!, toAccount: String!, from: String!, to: String! | When you know tx sender & receiver addresses, unix timestamp range & want to find out how many tx(s) were sent from sender to receiver in that certain timespan
`transactionsBetweenAccountsByTimeRange` | fromAccount: String!, toAccount: String!, from: String!, to: String! | When you know tx sender & receiver addresses, unix timestamp range & want to find out all tx(s) that were sent from sender to receiver in that certain timespan
`contractsCreatedFromAccountByNumberRange` | account: String!, from: String!, to: String! | When you know EOA's _( externally owned account )_ address & want to find out all contracts created by that account in block number range
`contractsCreatedFromAccountByTimeRange` | account: String!, from: String!, to: String! | When you know EOA's _( externally owned account )_ address & want to find out all contracts created by that account in certain time span
`transactionFromAccountWithNonce` | account: String!, nonce: String! | When you have EOA's address & nonce value of it, you can pin point to that tx. This can be used to iterate through all tx(s) from this account, by updating nonce.

---

### Historical Event Data ( GraphQL API )

**Path: `/v1/graphql`**

**Method: `POST`**

```graphql
type Query {
    eventsFromContractByNumberRange(contract: String!, from: String!, to: String!): [Event!]!
    eventsFromContractByTimeRange(contract: String!, from: String!, to: String!): [Event!]!
    eventsByBlockHash(hash: String!): [Event!]!
    eventsByTxHash(hash: String!): [Event!]!
    eventsFromContractWithTopicsByNumberRange(contract: String!, from: String!, to: String!, topics: [String!]!): [Event!]!
    eventsFromContractWithTopicsByTimeRange(contract: String!, from: String!, to: String!, topics: [String!]!): [Event!]!
    lastXEventsFromContract(contract: String!, x: Int!): [Event!]!
    eventByBlockHashAndLogIndex(hash: String!, index: String!): Event!
    eventByBlockNumberAndLogIndex(number: String!, index: String!): Event!
}
```

Response:

```graphql
type Event {
  origin: String!
  index: String!
  topics: [String!]!
  data: String!
  txHash: String!
  blockHash: String!
}
```

Method | Parameters | Possible use case
--- | --- | ---
`eventsFromContractByNumberRange` | contract: String!, from: String!, to: String! | When you've one contract address, block number range & you want to find out all events emitted by that contract in given block range
`eventsFromContractByTimeRange` | contract: String!, from: String!, to: String! | When you know contract address, unix time stamp range & you want to find out all events emitted by that contract in given timespan
`eventsByBlockHash` | hash: String! | When you've block hash & want to find out all events emitted in tx(s) packed in that block
`eventsByTxHash` | hash: String! | When you've txHash & want to find out all events emitted during execution of that tx
`eventsFromContractWithTopicsByNumberRange` | contract: String!, from: String!, to: String!, topics: [String!]! | When you've smart contract address, block number range & an ordered list of event log's topic signature(s), you can find out all events emitted by that contract with specific signature(s) in block range
`eventsFromContractWithTopicsByTimeRange` | contract: String!, from: String!, to: String!, topics: [String!]! | When you've smart contract address, unix time stamp range & an ordered list of event log's topic signature(s), you can find out all events emitted by that contract with specific signature(s) in given timespan
`lastXEventsFromContract` | contract: String!, x: Int! | When you know just contract address & want to find out last **X** events emitted by that contract **[ Very useful sometimes 😅 ]**
`eventByBlockHashAndLogIndex` | hash: String!, index: String! | When you know block hash, index of event log in block & want to get back specific event in that position
`eventByBlockHashAndLogIndex` | number: String!, index: String! | When you know block number, index of event log in block & want to get back specific event in that position

---

> GraphQL Playground : **/v1/graphql-playground**

---

### Real time notification for mined blocks

For listening to blocks getting mined, connect to `/v1/ws` endpoint using websocket client library & once connected, users need to send **subscription** request with payload _( JSON encoded )_

```json
{
    "name": "block",
    "type": "subscribe",
}
```

Subscription confirmeation response _( JSON encoded )_

```json
{
    "code": 1,
    "message": "Subscribed to `block`"
}
```

Real-time notification about new blocks getting mined:

```json
{
  "hash": "0x08f50b4795667528f6c0fdda31a0d270aae60dbe7bc4ea950ae1f71aaa01eabc",
  "number": 7015086,
  "time": 1605328635,
  "parentHash": "0x5ec0faff8b48e201e366a3f6c505eb274904e034c1565da2241f1327e9bad459",
  "difficulty": "6",
  "gasUsed": 78746,
  "gasLimit": 20000000,
  "nonce": 0,
  "miner": "0x0000000000000000000000000000000000000000",
  "size": 1044,
  "txRootHash": "0x088d6142b1d79803c851b1d839888b1e9f26c31e1266b4e221121f2cd8e85f86",
  "receiptRootHash": "0xca3949d52f113935ac08bae15e0816cd0472f01590f0fe0b65584bfb3aa324a6"
}
```

Cancel subscription:

```json
{
    "name": "block",
    "type": "unsubscribe",
    "apiKey": "0x..."
}
```

Unsubscription confirmation response:

```json
{
    "code": 1,
    "message": "Unsubscribed from `block`"
}
```

### Real time notification for transactions

```json
{
    "name": "transaction/<from-address>/<to-address>",
    "type": "subscribe",
}
```

**Examples :**

- Any transaction

```json
{
    "name": "transaction/*/*",
    "type": "subscribe",
    "apiKey": "0x..."
}
```

- Fixed `from` field **[ tx originated `from` account ]**

```json
{
    "name": "transaction/0x4774fEd3f2838f504006BE53155cA9cbDDEe9f0c/*",
    "type": "subscribe",
    "apiKey": "0x..."
}
```

- Fixed `to` field **[ tx targeted `to` account ]**

```json
{
    "name": "transaction/*/0x4774fEd3f2838f504006BE53155cA9cbDDEe9f0c",
    "type": "subscribe",
    "apiKey": "0x..."
}
```

- Fixed `from` & `to` field **[ tx `from` -> `to` account ]**

```json
{
    "name": "transaction/0xc9D50e0a571aDd06C7D5f1452DcE2F523FB711a1/0x4774fEd3f2838f504006BE53155cA9cbDDEe9f0c",
    "type": "subscribe",
    "apiKey": "0x..."
}
```
Subscription confirmation response _( JSON encoded )_

```json
{
    "code": 1,
    "message": "Subscribed to `transaction`",
    "apiKey": "0x..."
}
```

Real-time notification response for transaction subscription:

```json
{
  "hash": "0x08cfda79bd68ad280c7786e5dd349ab81981c52ea5cdd8e31be0a4b54b976555",
  "from": "0xc9D50e0a571aDd06C7D5f1452DcE2F523FB711a1",
  "to": "0x4774fEd3f2838f504006BE53155cA9cbDDEe9f0c",
  "contract": "",
  "value": "",
  "data": "0x35086d290000000000000000000000000000000000000000000000000000000000000360",
  "gas": 200000,
  "gasPrice": "1000000000",
  "cost": "200000000000000",
  "nonce": 19899,
  "state": 1,
  "blockHash": "0xc29170d33141602a95b915c954c1068a380ef5169178eef2538beb6edb005810"
}
```

Cancel subscription:

```json
{
    "name": "transaction/<from-address>/<to-address>",
    "type": "unsubscribe",
    "apiKey": "0x..."
}
```

Unsubscription confirmation response:

```json
{
    "code": 1,
    "message": "Unsubscribed from `transaction`"
}
```

### Real-time notification for events

```json
{
    "name": "event/<contract-address>/<topic-0-signature>/<topic-1-signature>/<topic-2-signature>/<topic-3-signature>",
    "type": "subscribe",
    "apiKey": "0x..."
}
```

**Examples :**

- Any event emitted by any smart contract in network

```json
{
    "name": "event/*/*/*/*/*",
    "type": "subscribe",
    "apiKey": "0x..."
}
```

- Any event emitted by one specific smart contract

```json
{
    "name": "event/0xcb3fA413B23b12E402Cfcd8FA120f983FB70d8E8/*/*/*/*",
    "type": "subscribe",
    "apiKey": "0x..."
}
```

- Specific event emitted by one specific smart contract

```json
{
    "name": "event/0xcb3fA413B23b12E402Cfcd8FA120f983FB70d8E8/0x2ab93f65628379309f36cb125e90d7c902454a545c4f8b8cb0794af75c24b807/*/*/*",
    "type": "subscribe",
    "apiKey": "0x..."
}
```

- Specific event emitted by any smart contract in network

```json
{
    "name": "event/*/0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef/*/*/*",
    "type": "subscribe",
    "apiKey": "0x..."
}
```

Subscription confirmation JSON encoded response

```json
{
    "code": 1,
    "message": "Subscribed to `event`"
}
```

Real-time notification you about every event emitted by smart contracts, to which subscribed to:

```json
{
  "origin": "0x0000000000000000000000000000000000001010",
  "index": 3,
  "topics": [
    "0x4dfe1bbbcf077ddc3e01291eea2d5c70c2b422b415d95645b9adcfd678cb1d63",
    "0x0000000000000000000000000000000000000000000000000000000000001010",
    "0x0000000000000000000000004d31abd8533c00436b2145795cc4cef207c3364f",
    "0x00000000000000000000000042eefcda06ead475cde3731b8eb138e88cd0bac3"
  ],
  "data": "0x0000000000000000000000000000000000000000000000000000454b2247e2000000000000000000000000000000000000000000000000001a96ae0b49dfc60000000000000000000000000000000000000000000000003a0df005a45c3dd5dd0000000000000000000000000000000000000000000000001a9668c02797e40000000000000000000000000000000000000000000000003a0df04aef7e85b7dd",
  "txHash": "0xfdc5a29fdd57a53953a542f4c46b0ece5423227f26b1191e58d32973b4d81dc9",
  "blockHash": "0x08e9ac45e4041a4309c6f5dd42b0fc78e00ca0cb8603965465206b22a63d07fb"
}
```

Cancel subscription:

```json
{
    "name": "event/<contract-address>/<topic-0-signature>/<topic-1-signature>/<topic-2-signature>/<topic-3-signature>",
    "type": "unsubscribe",
    "apiKey": "0x..."
}
```

Unsubscription confirmation response:

```json
{
    "code": 1,
    "message": "Unsubscribed from `event`"
}
```

> Note: If graceful unsubscription not done, if client unreachable, client subscription will get removed

<!-- omit in toc -->
## Notes:

- Code is written in Go

<!-- omit in toc -->
### Thought process and code design
- Concurrency support using event request queue

<!-- omit in toc -->
### Technology choices
- Postgres for persistant local storage
- Redis for pubsub
- graphql for api
- go-ethereum ethclient for blockchain communication

<!-- omit in toc -->
### How would you keep the data set up to date?
- ethclient websocket listening for new blocks

<!-- omit in toc -->
### How would you expose the stored data to customers in an easy-to-query API?
- Rest API using GraphQL

<!-- omit in toc -->
### How would you handle security of the API?
- postgres and redis authentication
- API key for users
- API delivery stats and history

<!-- omit in toc -->
### How would you improve the performance of your approach?
- First vertical scaling
- Then horizontal scaling with transition to more distributed architecture
- Redis to Kafka for pubsub
- DB sharding by blocks / tx and event assoicated address indexing
- Load balancer
- API key and user session management with in-memory caching of api requests and responses
- All historical data query requests to require ApiKey
- All real-time event subscription & unsubscription requests to require ApiKey
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