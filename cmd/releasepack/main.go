package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"dell-ecs-metrics-exporter/internal/releasepack"
)

func main() {
	source := flag.String("source", "", "source directory")
	output := flag.String("output", "", "output .tar.gz path")
	prefix := flag.String("prefix", "", "single archive root directory")
	epoch := flag.Int64("epoch", 0, "archive modification time as Unix epoch")
	flag.Parse()
	if flag.NArg() != 0 || *source == "" || *output == "" || *prefix == "" {
		fmt.Fprintln(os.Stderr, "source, output, and prefix are required")
		os.Exit(2)
	}
	if *epoch <= 0 {
		if value := os.Getenv("SOURCE_DATE_EPOCH"); value != "" {
			parsed, err := strconv.ParseInt(value, 10, 64)
			if err != nil || parsed <= 0 {
				fmt.Fprintln(os.Stderr, "SOURCE_DATE_EPOCH must be a positive Unix timestamp")
				os.Exit(2)
			}
			*epoch = parsed
		} else {
			fmt.Fprintln(os.Stderr, "epoch or SOURCE_DATE_EPOCH is required")
			os.Exit(2)
		}
	}
	if err := releasepack.Pack(
		*source, *output, *prefix, time.Unix(*epoch, 0).UTC(),
	); err != nil {
		fmt.Fprintf(os.Stderr, "create archive: %v\n", err)
		os.Exit(1)
	}
}
