package cli

import (
	"flag"
	"fmt"
	"strings"

	"github.com/RafaSG13/Pandora/internal/gen"
)

const (
	defaultGenLength  = 24
	defaultGenCharset = "alnum"
	maxGenCount       = 100
)

// cmdGen prints random secrets to stdout. Nothing is stored, and a generated
// value is only ever written to stdout, never to stderr.
func (a *app) cmdGen(args []string) error {
	names := charsetNames()
	usage := "gen [--length N] [--charset " + strings.Join(names, "|") + "] [--count N]"
	fs := flag.NewFlagSet("gen", flag.ContinueOnError)
	length := fs.Int("length", defaultGenLength, fmt.Sprintf("length of each secret, %d to %d", gen.MinLength, gen.MaxLength))
	charset := fs.String("charset", defaultGenCharset, "alphabet: "+strings.Join(names, ", "))
	count := fs.Int("count", 1, fmt.Sprintf("how many secrets to print, 1 to %d", maxGenCount))
	if _, err := a.parse("gen", usage, fs, args, 0, 0); err != nil {
		return err
	}

	if *length < gen.MinLength || *length > gen.MaxLength {
		return usageErrorf("gen: --length must be between %d and %d, got %d", gen.MinLength, gen.MaxLength, *length)
	}
	cs, ok := gen.Lookup(*charset)
	if !ok {
		return usageErrorf("gen: unknown charset %q (valid: %s)", *charset, strings.Join(names, ", "))
	}
	if *count < 1 || *count > maxGenCount {
		return usageErrorf("gen: --count must be between 1 and %d, got %d", maxGenCount, *count)
	}

	// Generate everything before printing so a failure never leaves partial output.
	secrets := make([]string, 0, *count)
	for range *count {
		s, err := gen.Generate(cs, *length)
		if err != nil {
			return fmt.Errorf("generating a secret: %w", err)
		}
		secrets = append(secrets, s)
	}
	_, err := fmt.Fprintln(a.out, strings.Join(secrets, "\n"))
	return err
}

// charsetNames lists the charset keys accepted by --charset.
func charsetNames() []string {
	names := make([]string, 0, len(gen.Charsets))
	for _, cs := range gen.Charsets {
		names = append(names, cs.Key)
	}
	return names
}
