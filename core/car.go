package core

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"

	"github.com/application-research/edge-ur/utils"
	commcid "github.com/filecoin-project/go-fil-commcid"
	commp "github.com/filecoin-project/go-fil-commp-hashhash"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
)

type CommpResult struct {
	commp     string
	pieceSize uint64
}

type Result struct {
	PayloadCid      string                       `json:"cid"`
	PieceCommitment PieceCommitment              `json:"piece_commitment"`
	Size            uint64                       `json:"size"`
	Miner           string                       `json:"miner"`
	CidMap          map[string]utils.CidMapValue `json:"cid_map"`
}

type PieceCommitment struct {
	PieceCID          string `json:"piece_cid"`
	PaddedPieceSize   uint64 `json:"padded_piece_size"`
	UnpaddedPieceSize uint64 `json:"unpadded_piece_size"`
}

type Input []utils.Finfo

type CarHeader struct {
	Roots   []cid.Cid
	Version uint64
}

func init() {
	cbor.RegisterCborType(CarHeader{})
}

const BufSize = (4 << 20) / 128 * 127

type CarParam struct {
	SourceInput    string
	SplitSizeInput string
	OutDir         string
	IncludeCommp   bool
}

func ChunkFileToCar(carParam CarParam) (Result, error) {
	ctx := context.Background()
	sourceInput := carParam.SourceInput
	splitSizeInput := carParam.SplitSizeInput
	outDir := carParam.OutDir
	includeCommp := carParam.IncludeCommp

	if _, err := os.Stat(outDir); os.IsNotExist(err) {
		return Result{}, err
	}
	var input Input
	var outputs []Result
	if splitSizeInput != "" {
		splitSizeA, err := strconv.Atoi(splitSizeInput)
		if err != nil {
			return Result{}, err
		}
		splitSize := int64(splitSizeA)

		err = filepath.Walk(sourceInput, func(sourcePath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}

			chunks := (info.Size() + splitSize - 1) / splitSize
			sourceFile, err := os.Open(sourcePath)
			if err != nil {
				return err
			}

			fileName := info.Name()
			chunkDir := filepath.Join(outDir, fileName)
			err = os.MkdirAll(chunkDir, 0755)
			if err != nil {
				return err
			}

			for i := int64(0); i < chunks; i++ {
				chunkFileName := fmt.Sprintf("%s_%04d", fileName, i)
				chunkFilePath := filepath.Join(chunkDir, chunkFileName)
				chunkFile, err := os.Create(chunkFilePath)
				if err != nil {
					return err
				}

				start := i * splitSize
				end := (i + 1) * splitSize
				if end > info.Size() {
					end = info.Size()
				}
				_, err = sourceFile.Seek(start, 0)
				if err != nil {
					return err
				}
				written, err := io.CopyN(chunkFile, sourceFile, end-start)
				if err != nil && err != io.EOF {
					return err
				}
				input = append(input, utils.Finfo{
					Path:  chunkFilePath,
					Size:  written,
					Start: 0,
					End:   written,
				})
				outFilename := uuid.New().String() + ".car"
				outPath := path.Join(outDir, outFilename)
				carF, err := os.Create(outPath)
				if err != nil {
					return err
				}
				cp := new(commp.Calc)
				writer := bufio.NewWriterSize(io.MultiWriter(carF, cp), BufSize)
				_, cid, cidMap, err := utils.GenerateCar(ctx, input, "", "", writer)
				if err != nil {
					return err
				}
				err = writer.Flush()
				if err != nil {
					return err
				}
				output := Result{
					PayloadCid: cid,
					CidMap:     cidMap,
				}

				if includeCommp {
					rawCommP, pieceSize, err := cp.Digest()
					if err != nil {
						return err
					}
					commCid, err := commcid.DataCommitmentV1ToCID(rawCommP)
					if err != nil {
						return err
					}
					err = os.Rename(outPath, path.Join(outDir, commCid.String()+".car"))
					if err != nil {
						return err
					}
					output.PieceCommitment.PieceCID = commCid.String()
					output.PieceCommitment.PaddedPieceSize = pieceSize
					output.Size = uint64(written)
				}
				outputs = append(outputs, output)
			}
			return nil
		})
		var buffer bytes.Buffer
		err = utils.PrettyEncode(outputs, &buffer)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(buffer.String())
		if err != nil {
			panic(err)
		}
	} else {
		stat, err := os.Stat(sourceInput)
		if err != nil {
			return Result{}, err
		}
		if stat.IsDir() {
			err := filepath.Walk(sourceInput, func(sourcePath string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() {
					return nil
				}
				input = append(input, utils.Finfo{
					Path:  sourcePath,
					Size:  info.Size(),
					Start: 0,
					End:   info.Size(),
				})
				outFilename := uuid.New().String() + ".car"
				outPath := path.Join(outDir, outFilename)
				carF, err := os.Create(outPath)
				if err != nil {
					return err
				}
				cp := new(commp.Calc)
				writer := bufio.NewWriterSize(io.MultiWriter(carF, cp), BufSize)
				_, cid, cidMap, err := utils.GenerateCar(ctx, input, "", "", writer)
				if err != nil {
					return err
				}
				err = writer.Flush()
				if err != nil {
					return err
				}

				output := Result{
					PayloadCid: cid,
					CidMap:     cidMap,
				}

				if includeCommp {
					rawCommP, pieceSize, err := cp.Digest()
					if err != nil {
						return err
					}
					commCid, err := commcid.DataCommitmentV1ToCID(rawCommP)
					if err != nil {
						return err
					}
					err = os.Rename(outPath, path.Join(outDir, commCid.String()+".car"))
					if err != nil {
						return err
					}
					output.PieceCommitment.PieceCID = commCid.String()
					output.PieceCommitment.PaddedPieceSize = pieceSize
					output.Size = uint64(info.Size())
				}
				outputs = append(outputs, output)
				return nil
			})

			var buffer bytes.Buffer
			err = utils.PrettyEncode(outputs, &buffer)
			if err != nil {
				fmt.Println(err)
			}
			fmt.Println(buffer.String())
			return Result{}, err
			if err != nil {
				return Result{}, err
			}
		} else {
			input = append(input, utils.Finfo{
				Path:  sourceInput,
				Size:  stat.Size(),
				Start: 0,
				End:   stat.Size(),
			})
			outFilename := uuid.New().String() + ".car"
			outPath := path.Join(outDir, outFilename)
			carF, err := os.Create(outPath)
			if err != nil {
				return Result{}, err
			}
			cp := new(commp.Calc)
			writer := bufio.NewWriterSize(io.MultiWriter(carF, cp), BufSize)
			_, cid, cidMap, err := utils.GenerateCar(ctx, input, "", "", writer)
			if err != nil {
				return Result{}, err
			}
			err = writer.Flush()
			if err != nil {
				return Result{}, err
			}
			output := Result{
				PayloadCid: cid,
				CidMap:     cidMap,
			}

			if includeCommp {
				rawCommP, pieceSize, err := cp.Digest()
				if err != nil {
					return Result{}, err
				}
				commCid, err := commcid.DataCommitmentV1ToCID(rawCommP)
				if err != nil {
					return Result{}, err
				}
				err = os.Rename(outPath, path.Join(outDir, commCid.String()+".car"))
				if err != nil {
					return Result{}, err
				}
				output.PieceCommitment.PieceCID = commCid.String()
				output.PieceCommitment.PaddedPieceSize = pieceSize
				output.Size = uint64(stat.Size())
			}
			outputs = append(outputs, output)
			if err != nil {
				return Result{}, err
			}
			var buffer bytes.Buffer
			err = utils.PrettyEncode(output, &buffer)
			if err != nil {
				fmt.Println(err)
			}
			fmt.Println(buffer.String())
		}
		return Result{}, nil
	}

	return Result{}, nil
}
