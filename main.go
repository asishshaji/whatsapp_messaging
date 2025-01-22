package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mdp/qrterminal"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

func eventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		fmt.Println("Received a message!", v.Message.GetConversation())
	}
}

func pdfToBytes(filePath string) ([]byte, error) {
	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()

	// Get the file size
	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("error getting file stats: %v", err)
	}

	// Read the file into a byte slice
	bytes := make([]byte, stat.Size())
	_, err = io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %v", err)
	}

	return bytes, nil
}

func main() {
	dbLog := waLog.Stdout("Database", "DEBUG", true)
	container, err := sqlstore.New("sqlite3", "file:examplestore.db?_foreign_keys=on", dbLog)
	if err != nil {
		panic(err)
	}
	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		panic(err)
	}
	clientLog := waLog.Stdout("Client", "DEBUG", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)
	client.AddEventHandler(eventHandler)

	if client.Store.ID == nil {
		// No ID stored, new login
		qrChan, _ := client.GetQRChannel(context.Background())
		err = client.Connect()
		if err != nil {
			panic(err)
		}
		for evt := range qrChan {
			if evt.Event == "code" {
				// Render the QR code here
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
				// or just manually `echo 2@... | qrencode -t ansiutf8` in a terminal
				fmt.Println("QR code:", evt.Code)
			} else {
				fmt.Println("Login event:", evt.Event)
			}
		}
	} else {
		// Already logged in, just connect
		err = client.Connect()
		if err != nil {
			panic(err)
		}
	}

	// Listen to Ctrl+C (you can also do something else that prevents the program from exiting)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	parsedJID, err := types.ParseJID("911234567890@s.whatsapp.net")
	if err != nil {
		fmt.Println("[ASISH] ERROR PARSING", err)
	}

	pdfBytes, _ := pdfToBytes("dummy.pdf")

	up, err := client.Upload(context.Background(), pdfBytes, whatsmeow.MediaDocument)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	// sends your pdf message, replace DocumentMessage with ConverstationMessage to send normal chat
	res, err := client.SendMessage(context.Background(),
		parsedJID, &waE2E.Message{
			DocumentMessage: &waE2E.DocumentMessage{
				Title:         proto.String("TEST PDF"),
				Mimetype:      proto.String("application/pdf"),
				URL:           &up.URL,
				DirectPath:    &up.DirectPath,
				MediaKey:      up.MediaKey,
				FileSHA256:    up.FileSHA256,
				FileEncSHA256: up.FileEncSHA256,
				FileLength:    &up.FileLength,
			},
		})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	_ = res
	client.Disconnect()
}
