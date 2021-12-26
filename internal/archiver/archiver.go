package archiver

import (
	"fmt"
	"time"

	"github.com/emersion/go-imap"
	move "github.com/emersion/go-imap-move"
	"github.com/emersion/go-imap/client"
	"github.com/pkg/errors"
)

type Archiver struct {
	sourceMailbox        string
	archiveMailboxFormat string
	archiveTimeFormat    string
	noNewerThan          time.Time
	dryRun               bool

	client *client.Client
	mover  *move.Client

	msgToMove map[string]*imap.SeqSet
}

func New(
	mailHost string,
	mailUser string,
	mailPass string,
	sourceMailbox string,
	archiveMailboxFormat string,
	archiveTimeFormat string,
) (*Archiver, error) {

	a := &Archiver{
		sourceMailbox:        sourceMailbox,
		archiveMailboxFormat: archiveMailboxFormat,
		archiveTimeFormat:    archiveTimeFormat,
		msgToMove:            make(map[string]*imap.SeqSet),

		//dryRun: true,
	}
	var err error

	if a.client, err = client.DialTLS(mailHost, nil); err != nil {
		return nil, errors.Wrapf(err, "dialing %s", mailHost)
	}

	if a.noNewerThan, err = time.Parse("2006-01", "2016-02"); err != nil {
		return nil, errors.Wrap(err, "parsing time")
	}

	// a.client.SetDebug(os.Stdout)

	if err := a.client.Login(mailUser, mailPass); err != nil {
		return nil, errors.Wrapf(err, "logging in as %s", mailUser)
	}

	a.mover = move.NewClient(a.client)
	moveSupported, err := a.mover.SupportMove()
	if err != nil {
		return nil, errors.Wrap(err, "checking if move is supported")
	}
	if !moveSupported {
		return nil, errors.New("move not supported")
	}

	return a, nil

}

func (a *Archiver) Fetch() error {
	// select mailbox
	mbox, err := a.client.Select(a.sourceMailbox, false)
	if err != nil {
		return errors.Wrapf(err, "selecting mailbox %s", a.sourceMailbox)
	}

	// Enmerate everything
	from := uint32(1)
	to := mbox.Messages
	seqset := new(imap.SeqSet)
	seqset.AddRange(from, to)

	messages := make(chan *imap.Message)
	done := make(chan error, 1)

	go func() {
		done <- a.client.Fetch(seqset, []imap.FetchItem{imap.FetchUid, imap.FetchFlags, imap.FetchInternalDate}, messages)
	}()

	go a.chanToMoveList(messages)

	if err := <-done; err != nil {
		return errors.Wrap(err, "fetching imap messages")
	}

	return nil
}

func (a *Archiver) chanToMoveList(messageChan chan *imap.Message) error {
	for {
		select {
		case msg := <-messageChan:
			if msg == nil {
				// fmt.Println("weird nil message")
				// return nil
				continue
			}

			if msg.InternalDate.Before(a.noNewerThan) {
				moveString := fmt.Sprintf(
					a.archiveMailboxFormat,
					a.sourceMailbox,
					msg.InternalDate.Format(a.archiveTimeFormat),
				)

				arr, ok := a.msgToMove[moveString]
				if !ok {
					arr = &imap.SeqSet{}
					a.msgToMove[moveString] = arr
				}

				// Use Uid, not SeqNum
				arr.AddNum(msg.Uid)

				//fmt.Printf("trying to add %d to %s\n", msg.Uid, timeString)
				//spew.Dump(msg)
				//fmt.Println(msg.Uid)
				//fmt.Println(msg.SeqNum)
				//fmt.Printf(archivePath, "fixme", msg.InternalDate.Format(timeFormat))
				//fmt.Print("\n")
			}
		}
	}
	return nil
}

func (a *Archiver) Move() error {
	for moveTarget, mvSeq := range a.msgToMove {
		if a.dryRun {
			fmt.Printf("Would move %d into %s\n", len(mvSeq.Set), moveTarget)
			continue
		}

		fmt.Printf("Moving %d into %s\n", len(mvSeq.Set), moveTarget)

		// a.client.SetDebug(os.Stdout)

		if err := a.client.Create(moveTarget); err != nil && fmt.Sprintf("%v", err) != "Mailbox already exists" {
			return errors.Wrapf(err, "creating mailbox %s", moveTarget)
		}

		if _, err := a.client.Select(a.sourceMailbox, false); err != nil {
			return errors.Wrapf(err, "selecting source mailbox %s", a.sourceMailbox)
		}

		if err := a.mover.UidMove(mvSeq, moveTarget); err != nil {
			fmt.Printf("Got error moving %d messages into %s\n", len(mvSeq.Set), moveTarget)
			return errors.Wrap(err, "moving messages")
		}

	}

	//fmt.Printf(archivePath, "fixme", msg.InternalDate.Format(timeFormat))

	//spew.Dump(a.msgToMove)
	return nil
}

func (a *Archiver) Close() error {
	return a.client.Logout()
}

func (a *Archiver) List() error {

	//a.client.SetDebug(os.Stdout)

	mailboxes := make(chan *imap.MailboxInfo)
	done := make(chan error, 1)

	go func() {
		done <- a.client.List("", "*", mailboxes)
	}()

	for m := range mailboxes {
		fmt.Printf("%s (using %s)\n", m.Name, m.Delimiter)
	}

	if err := <-done; err != nil {
		return errors.Wrap(err, "listing mailboxes")
	}

	return nil
}
