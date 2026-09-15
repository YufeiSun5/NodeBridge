package rulecheck

import (
	"encoding/json"
	"errors"
	"io"
	"os"

	"github.com/YufeiSun5/NodeBridge/internal/atomicfile"
)

type PairManifest struct {
	Version int            `json:"version"`
	Pairs   []ObservedPair `json:"pairs"`
}

func ReadPairManifest(path string) ([]ObservedPair, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 16<<20 {
		return nil, errors.New("bidirectional_manifest_size_invalid")
	}
	decoder := json.NewDecoder(io.LimitReader(f, 16<<20+1))
	decoder.DisallowUnknownFields()
	var manifest PairManifest
	if err := decoder.Decode(&manifest); err != nil {
		return nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, errors.New("bidirectional_manifest_trailing_data")
	}
	if manifest.Version != 1 || len(manifest.Pairs) == 0 || len(manifest.Pairs) > 256 {
		return nil, errors.New("bidirectional_manifest_version_or_count_invalid")
	}
	return manifest.Pairs, nil
}

func WritePairManifest(path string, pairs []ObservedPair) error {
	if len(pairs) == 0 || len(pairs) > 256 {
		return errors.New("bidirectional_manifest_version_or_count_invalid")
	}
	if _, err := BuildBidirectionalGraph(pairs); err != nil {
		return err
	}
	data, err := json.MarshalIndent(PairManifest{Version: 1, Pairs: pairs}, "", "  ")
	if err != nil {
		return err
	}
	if len(data) > 16<<20 {
		return errors.New("bidirectional_manifest_size_invalid")
	}
	return atomicfile.Write(path, data, 0o600)
}
