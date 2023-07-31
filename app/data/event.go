package data

import (
	"encoding/json"
	"log"

	"github.com/lib/pq"
)

// Event - Single event entity holder, extracted from db
type Event struct {
	Origin          string         `gorm:"column:origin"`
	Index           uint           `gorm:"column:index"`
	Topics          pq.StringArray `gorm:"column:topics;type:text[]"`
	Data            []byte         `gorm:"column:data"`
	TransactionHash string         `gorm:"column:txhash"`
	BlockHash       string         `gorm:"column:blockhash"`
}

// ToJSON - Encoding into JSON
func (e *Event) ToJSON() []byte {

	data, err := json.Marshal(e)
	if err != nil {
		log.Printf("[!] Failed to encode event to JSON : %s\n", err.Error())
		return nil
	}

	return data

}

// Events - A collection of event holder, to be delivered to client in this form
type Events struct {
	Events []*Event `json:"events"`
}

// ToJSON - Encoding to JSON
func (e *Events) ToJSON() []byte {

	data, err := json.Marshal(e)
	if err != nil {
		log.Printf("[!] Failed to encode events to JSON : %s\n", err.Error())
		return nil
	}

	return data

}
