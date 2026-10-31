package cli

import (
	"fmt"
	_ "image/gif" // images a code may be read from
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mahasenabheetha/codec/v2/internal/qr"
)

// codec qr (3.8.0): make a QR code (shown in the terminal, or saved as
// PNG or SVG) and read one from an image.

var (
	qrOut     string
	qrLevel   string
	qrScale   int
	qrWiFi    string
	qrPass    string
	qrSec     string
	qrHidden  bool
	qrTOTPAcc string
	qrTOTPIss string
	qrTOTPSec string
)

var qrCmd = &cobra.Command{
	Use:   "qr [text]",
	Short: "Make a QR code: shown in the terminal, or saved with -o as PNG or SVG",
	Long: `qr encodes text (the argument or stdin) as a QR code. Without -o it is
drawn in the terminal for a phone to scan; -o file.png or file.svg saves
it. --wifi makes a code that joins a network (--password, --security
WPA|WEP|none, --hidden); --totp-account and --totp-secret make an
authenticator setup link (--totp-issuer). --level L|M|Q|H sets how much
damage it survives (7, 15, 25, 30%).`,
	Example: `  codec qr https://example.com
  codec qr --wifi "Home" --password "correct horse" -o wifi.png
  codec qr --totp-account ann@example.com --totp-issuer Example --totp-secret JBSWY3DPEHPK3PXP`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var text string
		var err error
		switch {
		case qrWiFi != "":
			text, err = qr.WiFi{SSID: qrWiFi, Password: qrPass, Security: qrSec, Hidden: qrHidden}.Payload()
		case qrTOTPAcc != "" || qrTOTPSec != "":
			text, err = qr.TOTP{Issuer: qrTOTPIss, Account: qrTOTPAcc, Secret: qrTOTPSec}.Payload()
		default:
			text, err = readInput(args)
		}
		if err != nil {
			return err
		}
		c, err := qr.Encode(text, qrLevel)
		if err != nil {
			return err
		}
		switch strings.ToLower(filepath.Ext(qrOut)) {
		case "":
			fmt.Print(c.Terminal())
			return nil
		case ".svg":
			return os.WriteFile(qrOut, []byte(c.SVG()), 0o644)
		case ".png":
			b, err := c.PNG(qrScale)
			if err != nil {
				return err
			}
			return os.WriteFile(qrOut, b, 0o644)
		}
		return fmt.Errorf("-o ends in .png or .svg")
	},
}

var qrReadCmd = &cobra.Command{
	Use:   "read <image>",
	Short: "Read a QR code from a PNG, JPEG or GIF image",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}
		img, err := qr.Decode(data)
		if err != nil {
			return fmt.Errorf("%s: %w", args[0], err)
		}
		text, err := qr.Read(img)
		if err != nil {
			return err
		}
		m := qr.Explain(text)
		if m.Kind != "text" {
			fmt.Fprintln(cmd.ErrOrStderr(), "a "+m.Kind+" code")
			for _, f := range m.Fields {
				fmt.Fprintf(cmd.ErrOrStderr(), "  %-9s %s\n", f.Name, f.Value)
			}
		}
		return emit(text)
	},
}

func init() {
	qrCmd.Flags().StringVarP(&qrOut, "output", "o", "", "save as .png or .svg instead of drawing in the terminal")
	qrCmd.Flags().StringVar(&qrLevel, "level", "M", "error correction: L, M, Q or H")
	qrCmd.Flags().IntVar(&qrScale, "scale", 10, "PNG pixels per module")
	qrCmd.Flags().StringVar(&qrWiFi, "wifi", "", "a Wi-Fi network name (SSID) to join")
	qrCmd.Flags().StringVar(&qrPass, "password", "", "the Wi-Fi password")
	qrCmd.Flags().StringVar(&qrSec, "security", "WPA", "Wi-Fi security: WPA, WEP or none")
	qrCmd.Flags().BoolVar(&qrHidden, "hidden", false, "the network is hidden")
	qrCmd.Flags().StringVar(&qrTOTPAcc, "totp-account", "", "an authenticator account (often an email)")
	qrCmd.Flags().StringVar(&qrTOTPIss, "totp-issuer", "", "the authenticator issuer (the service)")
	qrCmd.Flags().StringVar(&qrTOTPSec, "totp-secret", "", "the TOTP secret, base32")
	qrCmd.AddCommand(qrReadCmd)
	rootCmd.AddCommand(qrCmd)
}
