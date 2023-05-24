package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/application-research/edge-ur/utils"
	"github.com/ipfs/go-cid"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/application-research/edge-ur/core"
)

type UploadCarToDeltaProcessor struct {
	CarBucket core.Bucket `json:"car_bucket"`
	RootCid   string      `json:"root_cid"`
	Processor
}

func NewUploadCarToDeltaProcessor(ln *core.LightNode, bucket core.Bucket, rootCid string) IProcessor {
	DELTA_UPLOAD_API = ln.Config.ExternalApi.ApiUrl
	REPLICATION_FACTOR = string(ln.Config.Common.ReplicationFactor)
	return &UploadCarToDeltaProcessor{
		bucket,
		rootCid,
		Processor{
			LightNode: ln,
		},
	}
}

func (r *UploadCarToDeltaProcessor) Info() error {
	panic("implement me")
}

func (r *UploadCarToDeltaProcessor) Run() error {

	// if network connection is not available or delta node is not available, then we need to skip and
	// let the upload retry consolidate the content until it is available

	maxRetries := 5
	retryInterval := 5 * time.Second

	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)

	partFile, err := writer.CreateFormFile("data", r.CarBucket.Cid)
	if err != nil {
		fmt.Println("CreateFormFile error: ", err)
		return nil
	}
	cidToGet, err := cid.Decode(r.CarBucket.Cid)
	if err != nil {
		fmt.Println("Error decoding cid: ", err)
		return nil
	}

	rootNd, err := r.LightNode.Node.DAGService.Get(context.Background(), cidToGet)
	if err != nil {
		fmt.Println("Error getting root node: ", err)
		return nil
	}
	bufFile := &bytes.Buffer{}
	for _, v := range rootNd.Links() {
		// get node
		lNd, err := r.LightNode.Node.GetFile(context.Background(), v.Cid)
		if err != nil {
			panic(err)
		}
		lNd.WriteTo(bufFile)
	}

	_, err = io.Copy(partFile, bufFile)
	if err != nil {
		fmt.Println("Copy error: ", err)
		return nil
	}

	repFactor := r.LightNode.Config.Common.ReplicationFactor
	partMetadata := fmt.Sprintf(`{"auto_retry":true,"miner":"%s","replication":%d}`, r.CarBucket.Miner, repFactor)
	writer.WriteField("metadata", partMetadata)

	if err = writer.Close(); err != nil {
		fmt.Println("Close error: ", err)
		return nil
	}

	req, err := http.NewRequest("POST",
		DELTA_UPLOAD_API+"/api/v1/deal/end-to-end",
		payload)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+r.CarBucket.RequestingApiKey)
	client := &http.Client{}
	var res *http.Response
	for j := 0; j < maxRetries; j++ {
		// Create a new http.Request instance
		clonedReq := &http.Request{}

		// Copy the properties from the original request
		*clonedReq = *req

		// Copy the headers
		clonedReq.Header = make(http.Header)
		for k, v := range req.Header {
			clonedReq.Header[k] = append([]string(nil), v...)
		}
		res, err = client.Do(clonedReq)
		if err != nil || res.StatusCode != http.StatusOK {
			time.Sleep(retryInterval)
			continue
		} else {
			if res.StatusCode == 200 {
				var dealE2EUploadResponse DealE2EUploadResponse
				body, err := ioutil.ReadAll(res.Body)
				if err != nil {
					fmt.Println(err)
					continue
				}
				err = json.Unmarshal(body, &dealE2EUploadResponse)
				if err != nil {
					fmt.Println(err)
					continue
				} else {
					if dealE2EUploadResponse.ContentID == 0 {
						continue
					} else {
						r.CarBucket.UpdatedAt = time.Now()
						r.CarBucket.Status = utils.STATUS_UPLOADED_TO_DELTA
						r.CarBucket.DeltaContentId = int64(dealE2EUploadResponse.ContentID)
						r.LightNode.DB.Save(&r.CarBucket)
						break
					}
				}
			}
		}
	}

	return nil
}
