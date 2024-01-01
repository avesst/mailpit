package cmd

import (
	"bytes"
	"io"
	"net/mail"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/axllent/mailpit/internal/logger"
	sendmail "github.com/axllent/mailpit/sendmail/cmd"
	"github.com/spf13/cobra"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var (
	ingestRecent int
)

// ingestCmd represents the ingest command
var ingestCmd = &cobra.Command{
	Use:   "ingest <file|folder> ...[file|folder]",
	Short: "Ingest a file or folder of emails for testing",
	Long: `Ingest a file or folder of emails for testing.

This command will scan the folder for emails and deliver them via SMTP to a running 
Mailpit server. Each email must be a separate file (eg: Maildir format, not mbox).
The --recent flag will only consider files with a modification date within the last X days.`,
	// Hidden: true,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var count int
		var total int
		var per100start = time.Now()
		p := message.NewPrinter(language.English)

		for _, a := range args {
			err := filepath.Walk(a,
				func(path string, info os.FileInfo, err error) error {
					if err != nil {
						logger.Log().Error(err)
						return nil
					}
					if !isFile(path) {
						return nil
					}

					info.ModTime()

					if ingestRecent > 0 && time.Now().Sub(info.ModTime()) > time.Duration(ingestRecent)*24*time.Hour {
						return nil
					}

					f, err := os.Open(filepath.Clean(path))
					if err != nil {
						logger.Log().Errorf("%s: %s", path, err.Error())
						return nil
					}
					defer f.Close() // #nosec

					body, err := io.ReadAll(f)
					if err != nil {
						logger.Log().Errorf("%s: %s", path, err.Error())
						return nil
					}

					msg, err := mail.ReadMessage(bytes.NewReader(body))
					if err != nil {
						logger.Log().Errorf("error parsing message body: %s", err.Error())
						return nil
					}

					recipients := []string{}
					// get all recipients in To, Cc and Bcc
					if to, err := msg.Header.AddressList("To"); err == nil {
						for _, a := range to {
							recipients = append(recipients, a.Address)
						}
					}
					if cc, err := msg.Header.AddressList("Cc"); err == nil {
						for _, a := range cc {
							recipients = append(recipients, a.Address)
						}
					}
					if bcc, err := msg.Header.AddressList("Bcc"); err == nil {
						for _, a := range bcc {
							recipients = append(recipients, a.Address)
						}
					}

					if sendmail.FromAddr == "" {
						if fromAddresses, err := msg.Header.AddressList("From"); err == nil {
							sendmail.FromAddr = fromAddresses[0].Address
						}
					}

					if len(recipients) == 0 {
						// Bcc
						recipients = []string{sendmail.FromAddr}
					}

					returnPath := strings.Trim(msg.Header.Get("Return-Path"), "<>")
					if returnPath == "" {
						if fromAddresses, err := msg.Header.AddressList("From"); err == nil {
							returnPath = fromAddresses[0].Address
						}
					}

					err = smtp.SendMail(sendmail.SMTPAddr, nil, returnPath, recipients, body)
					if err != nil {
						logger.Log().Errorf("error sending mail: %s (%s)", err.Error(), path)
						return nil
					}

					count++
					total++
					if count%100 == 0 {
						formatted := p.Sprintf("%d", total)
						logger.Log().Infof("[%s] 100 messages in %s", formatted, time.Since(per100start))

						per100start = time.Now()
					}

					return nil
				})
			if err != nil {
				logger.Log().Error(err)
			}
		}

	},
}

func init() {
	rootCmd.AddCommand(ingestCmd)

	ingestCmd.Flags().StringVarP(&sendmail.SMTPAddr, "smtp-addr", "S", sendmail.SMTPAddr, "SMTP server address")
	ingestCmd.Flags().IntVarP(&ingestRecent, "recent", "r", 0, "Only ingest messages from the last X days (default all)")
}

// IsFile returns if a path is a file
func isFile(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) || !info.Mode().IsRegular() {
		return false
	}

	return true
}
