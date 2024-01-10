package internal

import (
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/googleapi"
	"log"
	"strings"
	"sync"
	"time"

	"gmail-deleter/internal/database"
	"gmail-deleter/internal/models"
)

func Summarize(db database.Database) {
	fmt.Println("From,Count")

	report := db.Summarize()
	for _, r := range report {
		fmt.Print(r.From)
		fmt.Print(",")
		fmt.Print(r.Count)
		fmt.Println()
	}
}

func createThread(wg *sync.WaitGroup, t *gmail.Thread, db database.Database) {
	defer wg.Done()

	var thread models.Thread
	thread.Id = t.Id
	thread.Status = "NEW"

	err := db.Create(thread)
	if database.IsDup(err) {
		return
	}

	if err != nil {
		log.Fatal("Unable to retrieve threads: %v", err)
	}
}

func CountRecordsNeedingDelete(db database.Database, from string) *database.ItemList {
	return db.Count(bson.M{"status": "FETCHED", "from": from})
}

func DeleteEmailWorker(tid int, wg *sync.WaitGroup, gmail *gmail.Service, db database.Database, from string, deletionList *database.ItemList) {
	defer wg.Done()
	for k := deletionList.Pop(); k != nil; k = deletionList.Pop() {
		waitForWindow(1, db)
		thread := db.MoveToDeleting(k)
		if thread.Id == "" {
			continue
		}
		_, err := gmail.Users.Threads.
			Trash("me", thread.Id).
			Do()

		if err == nil {
			db.DeleteOne(thread.Id)
		} else {
			e, _ := err.(*googleapi.Error)
			if e.Code == 404 {
				db.DeleteOne(thread.Id)
			} else {
				log.Fatalf("Could not trash thread", e)
			}
		}
	}
}

func parseEmail(e string) string {
	if strings.Contains(e, "<") && strings.Contains(e, ">") {
		f := func(c rune) bool {
			return (c == '<') || (c == '>')
		}
		tokens := strings.FieldsFunc(e, f)

		if len(tokens) == 1 {
			return tokens[0]
		}
		return tokens[1]
	}

	return e
}

func waitForWindow(cost int, db database.Database) {
	for {
		canProcess := db.ReserveWindow(cost)

		if canProcess {
			break
		}
		log.Println("waiting for reserving a window...")
		time.Sleep(10 * time.Millisecond)
	}
}

func FetchEmailWorker(tid int, wg *sync.WaitGroup, gmail *gmail.Service, db database.Database) {
	defer wg.Done()
	for {
		waitForWindow(10, db)

		thread := db.FindOne(bson.M{"status": "NEW"}, "FETCHING_THREAD")
		if thread.Id == "" {
			// log.Println(tid, "Finished fetching")
			return
		}

		r, err := gmail.Users.Threads.
			Get("me", thread.Id).
			Do()

		if err != nil {
			log.Fatal("Unable to get gmail thread", err)
		}

		message := r.Messages[0]
		thread.Created = time.Unix(message.InternalDate/1000, 0)

		messagePart := message.Payload
		headers := messagePart.Headers

		// TODO: filter out chats

		for _, h := range headers {
			if strings.EqualFold(h.Name, "from") {
				thread.From = parseEmail(strings.ToLower(h.Value))
			}
			if strings.EqualFold(h.Name, "to") {
				thread.To = parseEmail(strings.ToLower(h.Value))
			}

			// TODO: BCC, subject, snippet for richer searching
		}

		thread.Status = "FETCHED"

		db.Populate(thread)
	}

}

func ListThreads(gmail *gmail.Service, db database.Database) {
	var wg sync.WaitGroup

	user := "me"
	nextPageToken := ""

	for hasPage := true; hasPage; {
		waitForWindow(10, db)

		// https://support.google.com/mail/answer/7190
		// https://developers.google.com/gmail/api/guides/filtering
		// -is:chat
		r, err := gmail.Users.Threads.
			List(user).
			MaxResults(500).
			PageToken(nextPageToken).
			Fields("nextPageToken", "threads/id").
			Do()

		if err != nil {
			log.Fatalf("Unable to retrieve threads: %v", err)
		}

		for _, l := range r.Threads {
			wg.Add(1)
			go createThread(&wg, l, db)
		}

		// TODO: save this token in case we want to resume
		nextPageToken = r.NextPageToken
		if nextPageToken == "" {
			hasPage = false
		}
	}

	wg.Wait()
}
