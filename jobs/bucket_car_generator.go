package jobs

import (
	"bytes"
	"context"
	"fmt"
	"github.com/application-research/edge-ur/core"
	"github.com/application-research/filclient"
	"github.com/filecoin-project/go-data-segment/datasegment"
	"github.com/filecoin-project/go-data-segment/util"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-cid"
	uio "github.com/ipfs/go-unixfs/io"
	"io"
)

// The BucketCarGenerator type has a Bucket field and implements the Processor interface.
// @property Bucket - The `Bucket` property is a field of type `core.Bucket`. It is likely used to store or retrieve data
// related to cars, such as their make, model, year, and other attributes. The `BucketCarGenerator` struct likely
// represents a component or module that is responsible for generating new
// @property {Processor}  - The `BucketCarGenerator` struct has two properties:
type BucketCarGenerator struct {
	Bucket core.Bucket
	Processor
}

func (g BucketCarGenerator) Info() error {
	panic("implement me")
}

func (g BucketCarGenerator) Run() error {
	g.GenerateCarForBucket(g.Bucket.Uuid)
	return nil
}

func NewBucketCarGenerator(ln *core.LightNode, bucketToProcess core.Bucket) IProcessor {
	return &BucketCarGenerator{
		bucketToProcess,
		Processor{
			LightNode: ln,
		},
	}
}

func (r *BucketCarGenerator) GenerateCarForBucket(bucketUuid string) {

	// create node and raw per file (layer them)
	var contentsToUpdateWithPieceInfo []core.Content
	r.LightNode.DB.Model(&core.Content{}).Where("bucket_uuid = ?", bucketUuid).Find(&contentsToUpdateWithPieceInfo)

	// for each content, generate a node and a raw
	dir := uio.NewDirectory(r.LightNode.Node.DAGService)
	dir.SetCidBuilder(GetCidBuilderDefault())

	// get the subPieceInfos
	var subPieceInfos []abi.PieceInfo
	var intTotalSize int64
	for _, c := range contentsToUpdateWithPieceInfo {
		cCid, err := cid.Decode(c.Cid)
		if err != nil {
			panic(err)
		}

		pieceCid, pieceSize, _, err := filclient.GeneratePieceCommitment(context.Background(), cCid, r.LightNode.Node.Blockstore)

		c.PieceCid = pieceCid.String()
		cielPow2Piece, err := util.CeilPow2(pieceSize)
		if err != nil {
			panic(err)
		}

		c.PieceSize = int64(cielPow2Piece)

		// add to the array
		subPieceInfos = append(subPieceInfos, abi.PieceInfo{
			Size:     abi.PaddedPieceSize(cielPow2Piece),
			PieceCID: pieceCid,
		})

		intTotalSize += int64(cielPow2Piece)
		fmt.Println("PieceCid1: ", c.PieceCid)
		fmt.Println("PieceSize1: ", c.PieceSize)

		r.LightNode.DB.Save(&c)
	}

	// generate the aggregate using the subpieceinfos
	totalSizePow2, err := util.CeilPow2(uint64(intTotalSize))
	if err != nil {
		panic(err)
	}
	agg, err := datasegment.NewAggregate(abi.PaddedPieceSize(totalSizePow2), subPieceInfos)
	if err != nil {
		panic(err)
	}

	var aggReaders []io.Reader
	var updateContentsForAgg []core.Content
	r.LightNode.DB.Model(&core.Content{}).Where("bucket_uuid = ?", bucketUuid).Find(&updateContentsForAgg)
	for _, cAgg := range updateContentsForAgg {
		cCidAgg, err := cid.Decode(cAgg.Cid)
		if err != nil {
			panic(err)
		}
		cDataAgg, errCData := r.LightNode.Node.GetFile(context.Background(), cCidAgg) // get the node
		if errCData != nil {
			panic(errCData)
		}
		aggReaders = append(aggReaders, cDataAgg)
	}

	rootReader, err := agg.AggregateObjectReader(aggReaders)
	if err != nil {
		panic(err)
	}

	aggNd, err := r.LightNode.Node.AddPinFile(context.Background(), rootReader, nil)
	if err != nil {
		panic(err)
	}

	var bucket core.Bucket
	r.LightNode.DB.Model(&core.Bucket{}).Where("uuid = ?", bucketUuid).First(&bucket)
	bucket.Cid = aggNd.Cid().String()
	bucket.RequestingApiKey = r.Bucket.RequestingApiKey
	bucket.Miner = "t017840"
	aggCid, err := agg.PieceCID()

	if err != nil {
		panic(err)
	}

	bucket.PieceCid = aggCid.String()
	bucket.PieceSize = int64(agg.DealSize)
	bucket.Status = "filled"
	bucket.Size = intTotalSize
	r.LightNode.DB.Save(&bucket)

	// get the proof for each piece
	var updatedContents []core.Content
	r.LightNode.DB.Model(&core.Content{}).Where("bucket_uuid = ?", bucketUuid).Find(&updatedContents)

	for _, cProof := range updatedContents {
		fmt.Println("PieceCid2: ", cProof.PieceCid)
		fmt.Println("PieceSize2: ", cProof.PieceSize)
		pieceCidStr, err := cid.Decode(cProof.PieceCid)
		if err != nil {
			panic(err)
		}

		pieceInfo := abi.PieceInfo{
			Size:     abi.PaddedPieceSize(cProof.PieceSize),
			PieceCID: pieceCidStr,
		}
		proofForEach, err := agg.ProofForPieceInfo(pieceInfo)
		if err != nil {
			panic(err)
		}
		//_, err = proofForEach.ComputeExpectedAuxData(datasegment.VerifierDataForPieceInfo(pieceInfo))
		//if err != nil {
		//	panic(err)
		//}

		//bucketPieceCid, _ := cid.Decode(bucket.PieceCid)
		//if aux.CommPa.String() != bucketPieceCid.String() {
		//	panic("commPa does not match")
		//}

		incW := &bytes.Buffer{}
		proofForEach.MarshalCBOR(incW)
		cProof.InclusionProof = incW.Bytes()

		r.LightNode.DB.Save(&cProof)
	}

	fmt.Println("Bucket CID: ", bucket.Cid)
	fmt.Println("Bucket Size: ", bucket.Size)
	fmt.Println("Bucket Piece CID: ", bucket.PieceCid)
	fmt.Println("Bucket Piece Size: ", bucket.PieceSize)

	job := CreateNewDispatcher()
	job.AddJob(NewUploadCarToDeltaProcessor(r.LightNode, bucket, bucket.Cid))
	job.Start(1)

}
