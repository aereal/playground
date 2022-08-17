package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/aereal/playground/enjoy-protobuf/definition/pb"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const (
	dataFileName = "livers.json"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	defFromFile := unmarshalFromFile()
	def := &pb.Definition{Livers: map[string]*pb.Liver{
		"lize": {Name: "Lize Helesta", Age: proto.Int32(17)},
		"mito": {Name: "Tsukino Mito", Age: proto.Int32(16)},
	}}
	diff := cmp.Diff(def, defFromFile, cmpopts.IgnoreUnexported(pb.Definition{}, pb.Liver{}))
	if diff == "" {
		return nil
	}
	fmt.Printf("(-want, +got):\n%s\n", diff)
	m := &protojson.MarshalOptions{Multiline: true}
	j, err := m.Marshal(def)
	if err != nil {
		return fmt.Errorf("protojson.Marshal: %w", err)
	}
	if err := ioutil.WriteFile(dataFileName, j, 0666); err != nil {
		return fmt.Errorf("ioutil.WriteFile: %w", err)
	}
	return nil
}

func unmarshalFromFile() *pb.Definition {
	input, err := ioutil.ReadFile(dataFileName)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return nil
	}
	var m pb.Definition
	if err := protojson.Unmarshal(input, &m); err != nil {
		return nil
	}
	return &m
}
