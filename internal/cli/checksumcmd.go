package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mahasenabheetha/codec/v2/internal/encode"
)

// codec checksum (3.8.0): files hashed as a stream, printed the way
// sha256sum does, or checked against an expected value or a checksums
// file.

var (
	sumAlgo   string
	sumAll    bool
	sumCheck  string
	sumExpect string
)

// fileDigests checksums one file (or stdin for "-").
func fileDigests(name string, algos ...string) ([]encode.Digest, error) {
	if name == "-" {
		d, _, err := encode.Checksum(os.Stdin, algos...)
		return d, err
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	d, _, err := encode.Checksum(f, algos...)
	return d, err
}

var checksumCmd = &cobra.Command{
	Use:   "checksum [file...]",
	Short: "Checksum files (sha256sum-style), or verify them against an expected value or a checksums file",
	Long: `checksum hashes files as a stream, any size, and prints one
"<hex>  <file>" line each (SHA-256 unless --algo; --all for every
algorithm). --expect checks one file against a value given as hex,
base64 or a sha256sum line. --check reads a checksums file (sha256sum or
BSD lines; the algorithm follows from each digest's length) and checks
every file it names (beside the checksums file, else from the current
folder); the exit code is 1
when one fails or is missing.`,
	Example: `  codec checksum codec_3.8.0_linux_amd64.tar.gz
  codec checksum --expect 9f86d081… codec.zip
  codec checksum --check checksums.txt`,
	RunE: func(cmd *cobra.Command, args []string) error {
		switch {
		case sumCheck != "":
			return checkList(cmd, sumCheck)
		case sumExpect != "":
			if len(args) != 1 {
				return fmt.Errorf("--expect checks one file")
			}
			return checkOne(cmd, args[0], sumExpect)
		}
		if len(args) == 0 {
			args = []string{"-"}
		}
		var out strings.Builder
		for _, name := range args {
			var algos []string
			if !sumAll {
				algos = []string{sumAlgo}
			}
			ds, err := fileDigests(name, algos...)
			if err != nil {
				return err
			}
			for _, d := range ds {
				if sumAll {
					fmt.Fprintf(&out, "%-8s %s  %s\n", d.Algorithm, d.Hex, name)
				} else {
					fmt.Fprintf(&out, "%s  %s\n", d.Hex, name)
				}
			}
		}
		return emit(strings.TrimRight(out.String(), "\n"))
	},
}

func checkOne(cmd *cobra.Command, name, want string) error {
	es, err := encode.ParseExpected(want)
	if err != nil {
		return fmt.Errorf("--expect: %w", err)
	}
	e := es[0]
	ds, err := fileDigests(name, e.Algorithm)
	if err != nil {
		return err
	}
	if _, equal := e.Match(ds); !equal {
		fmt.Fprintf(cmd.OutOrStdout(), "%s: FAILED (%s %s, expected %s)\n", name, e.Algorithm, ds[0].Hex, e.Hex)
		return exitCode(1)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s: OK (%s)\n", name, e.Algorithm)
	return nil
}

func checkList(cmd *cobra.Command, list string) error {
	src, err := os.ReadFile(list)
	if err != nil {
		return err
	}
	es, err := encode.ParseExpected(string(src))
	if err != nil {
		return fmt.Errorf("%s: %w", list, err)
	}
	dir := filepath.Dir(list)
	bad := 0
	for _, e := range es {
		if e.Name == "" {
			return fmt.Errorf("%s:%d: a checksum without a file name", list, e.Line)
		}
		// Beside the checksums file first (a release folder), then from
		// here, as sha256sum -c reads names.
		name := filepath.FromSlash(e.Name)
		ds, err := fileDigests(filepath.Join(dir, name), e.Algorithm)
		if err != nil && !filepath.IsAbs(name) && dir != "." {
			ds, err = fileDigests(name, e.Algorithm)
		}
		switch {
		case err != nil:
			bad++
			fmt.Fprintf(cmd.OutOrStdout(), "%s: MISSING\n", e.Name)
		case ds[0].Hex != e.Hex:
			bad++
			fmt.Fprintf(cmd.OutOrStdout(), "%s: FAILED\n", e.Name)
		default:
			fmt.Fprintf(cmd.OutOrStdout(), "%s: OK\n", e.Name)
		}
	}
	if bad > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "%d of %d did not match\n", bad, len(es))
		return exitCode(1)
	}
	return nil
}

func init() {
	checksumCmd.Flags().StringVar(&sumAlgo, "algo", "sha256", "algorithm: sha256, sha384, sha512, sha1 or md5")
	checksumCmd.Flags().BoolVar(&sumAll, "all", false, "every algorithm")
	checksumCmd.Flags().StringVar(&sumCheck, "check", "", "verify the files a checksums file lists")
	checksumCmd.Flags().StringVar(&sumExpect, "expect", "", "verify one file against this checksum")
	rootCmd.AddCommand(checksumCmd)
}
