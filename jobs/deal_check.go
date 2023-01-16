package jobs

import (
	"edge-ur/core"
	"encoding/json"
	"fmt"
	cid2 "github.com/ipfs/go-cid"
	"github.com/spf13/viper"
	"net/http"
	"time"
)

type DealCheckProcessor struct {
	Processor
}

type ContentStatus struct {
	Content struct {
		ID            int       `json:"id"`
		CreatedAt     time.Time `json:"createdAt"`
		UpdatedAt     time.Time `json:"updatedAt"`
		Cid           string    `json:"cid"`
		Name          string    `json:"name"`
		UserID        int       `json:"userId"`
		Description   string    `json:"description"`
		Size          int       `json:"size"`
		Type          int       `json:"type"`
		Active        bool      `json:"active"`
		Offloaded     bool      `json:"offloaded"`
		Replication   int       `json:"replication"`
		AggregatedIn  int       `json:"aggregatedIn"`
		Aggregate     bool      `json:"aggregate"`
		Pinning       bool      `json:"pinning"`
		PinMeta       string    `json:"pinMeta"`
		Replace       bool      `json:"replace"`
		Origins       string    `json:"origins"`
		Failed        bool      `json:"failed"`
		Location      string    `json:"location"`
		DagSplit      bool      `json:"dagSplit"`
		SplitFrom     int       `json:"splitFrom"`
		PinningStatus string    `json:"pinningStatus"`
		DealStatus    string    `json:"dealStatus"`
	} `json:"content"`
	Deals []struct {
		Deal struct {
			ID                  int         `json:"ID"`
			CreatedAt           time.Time   `json:"CreatedAt"`
			UpdatedAt           time.Time   `json:"UpdatedAt"`
			DeletedAt           interface{} `json:"DeletedAt"`
			Content             int         `json:"content"`
			UserID              int         `json:"user_id"`
			PropCid             string      `json:"propCid"`
			DealUUID            string      `json:"dealUuid"`
			Miner               string      `json:"miner"`
			DealID              int         `json:"dealId"`
			Failed              bool        `json:"failed"`
			Verified            bool        `json:"verified"`
			Slashed             bool        `json:"slashed"`
			FailedAt            time.Time   `json:"failedAt"`
			DtChan              string      `json:"dtChan"`
			TransferStarted     time.Time   `json:"transferStarted"`
			TransferFinished    time.Time   `json:"transferFinished"`
			OnChainAt           time.Time   `json:"onChainAt"`
			SealedAt            time.Time   `json:"sealedAt"`
			DealProtocolVersion string      `json:"deal_protocol_version"`
			MinerVersion        string      `json:"miner_version"`
		} `json:"deal"`
		Transfer     interface{} `json:"transfer"`
		OnChainState struct {
			SectorStartEpoch int `json:"sectorStartEpoch"`
			LastUpdatedEpoch int `json:"lastUpdatedEpoch"`
			SlashEpoch       int `json:"slashEpoch"`
		} `json:"onChainState"`
	} `json:"deals"`
	FailuresCount int `json:"failuresCount"`
}

func NewDealCheckProcessor(ln *core.LightNode) IProcessor {
	MODE = viper.Get("MODE").(string)
	UPLOAD_ENDPOINT = viper.Get("REMOTE_PIN_ENDPOINT").(string)
	DELETE_AFTER_DEAL_MADE = viper.Get("DELETE_AFTER_DEAL_MADE").(string)
	CONTENT_STATUS_CHECK_ENDPOINT = viper.Get("CONTENT_STATUS_CHECK_ENDPOINT").(string)
	return &DealCheckProcessor{
		Processor{
			LightNode: ln,
		},
	}
}

func (r *DealCheckProcessor) Info() error {
	//TODO implement me
	panic("implement me")
}

func (r *DealCheckProcessor) Run() error {
	// get the deal of the contents and update

	// get the contents that has estuary_request_id from the DB
	var contents []core.Content
	r.LightNode.DB.Where("estuary_request_id IS NOT NULL").Find(&contents)

	for _, content := range contents {

		req, _ := http.NewRequest("GET",
			CONTENT_STATUS_CHECK_ENDPOINT+"/"+content.EstuaryContentId, nil)

		client := &http.Client{}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+content.RequestingApiKey)
		res, err := client.Do(req)

		var contentStatus ContentStatus
		err = json.NewDecoder(res.Body).Decode(&contentStatus)
		if err != nil {
			fmt.Println(err)
			return err
		}

		if res.StatusCode != 202 {
			fmt.Println("error check estuary content id", res.StatusCode)
			continue
		}
	}
	return nil
}
func (r *DealCheckProcessor) deleteCidOnLocalNode(cidParam string) {
	// delete the cid on the local node
	cid, error := cid2.Decode(cidParam)

	if error != nil {
		panic(error)
	}
	r.LightNode.Node.Blockstore.DeleteBlock(*r.context, cid) //
}
