package db

import (
	"bytes"

	"github.com/lib/pq"
)

// Tabler - ...
type Tabler interface {
	TableName() string
}

// Blocks - Mined block info holder table model
type Blocks struct {
	Hash                string       `gorm:"column:hash;type:char(66);primaryKey"`
	Number              uint64       `gorm:"column:number;type:bigint;not null;unique;index:,sort:asc"`
	Time                uint64       `gorm:"column:time;type:bigint;not null;index:,sort:asc"`
	ParentHash          string       `gorm:"column:parenthash;type:char(66);not null"`
	Difficulty          string       `gorm:"column:difficulty;type:varchar;not null"`
	GasUsed             uint64       `gorm:"column:gasused;type:bigint;not null"`
	GasLimit            uint64       `gorm:"column:gaslimit;type:bigint;not null"`
	Nonce               string       `gorm:"column:nonce;type:varchar;not null"`
	Miner               string       `gorm:"column:miner;type:char(42);not null"`
	Size                float64      `gorm:"column:size;type:float(8);not null"`
	StateRootHash       string       `gorm:"column:stateroothash;type:char(66);not null"`
	UncleHash           string       `gorm:"column:unclehash;type:char(66);not null"`
	TransactionRootHash string       `gorm:"column:txroothash;type:char(66);not null"`
	ReceiptRootHash     string       `gorm:"column:receiptroothash;type:char(66);not null"`
	ExtraData           []byte       `gorm:"column:extradata;type:bytea"`
	Transactions        Transactions `gorm:"foreignKey:blockhash;constraint:OnDelete:CASCADE;"`
	Events              Events       `gorm:"foreignKey:blockhash;constraint:OnDelete:CASCADE;"`
}

// TableName - Overriding default table name
func (Blocks) TableName() string {
	return "blocks"
}

// SimilarTo - Checking whether two blocks are exactly similar or not
func (b *Blocks) SimilarTo(_b *Blocks) bool {
	return b.Hash == _b.Hash &&
		b.Number == _b.Number &&
		b.Time == _b.Time &&
		b.ParentHash == _b.ParentHash &&
		b.Difficulty == _b.Difficulty &&
		b.GasUsed == _b.GasUsed &&
		b.GasLimit == _b.GasLimit &&
		b.Nonce == _b.Nonce &&
		b.Miner == _b.Miner &&
		b.Size == _b.Size &&
		b.StateRootHash == _b.StateRootHash &&
		b.UncleHash == _b.UncleHash &&
		b.TransactionRootHash == _b.TransactionRootHash &&
		b.ReceiptRootHash == _b.ReceiptRootHash &&
		bytes.Equal(b.ExtraData, _b.ExtraData)
}

// Transactions - Blockchain transaction holder table model
type Transactions struct {
	Hash      string `gorm:"column:hash;type:char(66);primaryKey"`
	From      string `gorm:"column:from;type:char(42);not null;index"`
	To        string `gorm:"column:to;type:char(42);index"`
	Contract  string `gorm:"column:contract;type:char(42);index"`
	Value     string `gorm:"column:value;type:varchar"`
	Data      []byte `gorm:"column:data;type:bytea"`
	Gas       uint64 `gorm:"column:gas;type:bigint;not null"`
	GasPrice  string `gorm:"column:gasprice;type:varchar;not null"`
	Cost      string `gorm:"column:cost;type:varchar;not null"`
	Nonce     uint64 `gorm:"column:nonce;type:bigint;not null;index"`
	State     uint64 `gorm:"column:state;type:smallint;not null"`
	BlockHash string `gorm:"column:blockhash;type:char(66);not null;index"`
	Events    Events `gorm:"foreignKey:txhash;constraint:OnDelete:CASCADE;"`
}

// TableName - Overriding default table name
func (Transactions) TableName() string {
	return "transactions"
}

// Events - Events emitted from smart contracts to be held in this table
type Events struct {
	BlockHash       string         `gorm:"column:blockhash;type:char(66);not null;primaryKey"`
	Index           uint           `gorm:"column:index;type:integer;not null;primaryKey"`
	Origin          string         `gorm:"column:origin;type:char(42);not null;index"`
	Topics          pq.StringArray `gorm:"column:topics;type:text[];not null;index:,type:gin"`
	Data            []byte         `gorm:"column:data;type:bytea"`
	TransactionHash string         `gorm:"column:txhash;type:char(66);not null;index"`
}

// TableName - Overriding default table name
func (Events) TableName() string {
	return "events"
}

// PackedTransaction - All data that is stored in a tx, to be passed from
// tx data fetcher to whole block data persist handler function
type PackedTransaction struct {
	Tx     *Transactions
	Events []*Events
}

// PackedBlock - Whole block data to be persisted in a single
// database transaction to ensure data consistency, if something
// goes wrong in mid, whole persisting operation will get reverted
type PackedBlock struct {
	Block        *Blocks
	Transactions []*PackedTransaction
}
