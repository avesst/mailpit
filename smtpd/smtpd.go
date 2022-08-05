package smtpd

import (
	"bytes"
	"net"
	"net/mail"
	"regexp"

	"github.com/axllent/mailpit/config"
	"github.com/axllent/mailpit/logger"
	"github.com/axllent/mailpit/storage"
	s "github.com/mhale/smtpd"
)

func mailHandler(origin net.Addr, from string, to []string, data []byte) error {
	msg, err := mail.ReadMessage(bytes.NewReader(data))
	if err != nil {
		logger.Log().Errorf("error parsing message: %s", err.Error())
		return err
	}

	if _, err := storage.Store(storage.DefaultMailbox, data); err != nil {
		// Value with size 4800709 exceeded 1048576 limit
		re := regexp.MustCompile(`(Value with size \d+ exceeded \d+ limit)`)
		tooLarge := re.FindStringSubmatch(err.Error())
		if len(tooLarge) > 0 {
			logger.Log().Errorf("[db] error storing message: %s", tooLarge[0])
		} else {
			logger.Log().Errorf("[db] error storing message")
			logger.Log().Errorf(err.Error())
		}
		return err
	}

	subject := msg.Header.Get("Subject")
	logger.Log().Debugf("[smtp] received mail from %s for %s with subject %s", from, to[0], subject)
	return nil
}

// Listen starts the SMTPD server
func Listen() error {
	logger.Log().Infof("[smtp] starting on %s", config.SMTPListen)
	if err := s.ListenAndServe(config.SMTPListen, mailHandler, "Mailpit", ""); err != nil {
		return err
	}

	return nil
}
