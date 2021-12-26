package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/directionless/imap-mail-archiver/internal/archiver"
	"github.com/peterbourgon/ff"
)

func main() {
	flagset := flag.NewFlagSet("imap-mail-archiver", flag.ExitOnError)

	var (
		flMailHost          = flagset.String("host", "mail.messagingengine.com", "Mail server host name")
		flMailUser          = flagset.String("username", "seph@imapmail.org", "Mail server user name")
		flMailPass          = flagset.String("password", "", "Mail server password")
		flSourceMailbox     = flagset.String("mailbox", "INBOX", "Mailbox to archive mail from")
		flArchivePath       = flagset.String("archive-path", "INBOX.zzarchive.%s.%s", "Where is this mail being archived to")
		flArchiveTimeFormat = flagset.String("archive-time", "2006", "Format to convert the time to, in golang's date format")

		_ = flagset.String("config", "", "config file to parse options from (optional)")
	)

	ff.Parse(flagset, os.Args[1:],
		ff.WithConfigFileFlag("config"),
		ff.WithConfigFileParser(ff.PlainParser),
		ff.WithEnvVarPrefix("IMAP_MAIL_ARCHIVER"),
	)

	archiver, err := archiver.New(
		fmt.Sprintf("%s:993", *flMailHost),
		*flMailUser,
		*flMailPass,
		*flSourceMailbox,
		*flArchivePath,
		*flArchiveTimeFormat,
	)
	if err != nil {
		log.Fatal(err)
	}

	defer archiver.Close()

	//if err := archiver.List(); err != nil {
	//	log.Fatal(err)
	//}

	if err := archiver.Fetch(); err != nil {
		log.Fatal(err)
	}

	if err := archiver.Move(); err != nil {
		log.Fatal(err)
	}

}
