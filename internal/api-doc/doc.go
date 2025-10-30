package api_doc

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/SENERGY-Platform/api-docs-provider/lib/client"
)

func PublishAsyncapiDoc() {
	docksProviderUrl := os.Getenv("API_DOCS_PROVIDER_BASE_URL")
	if docksProviderUrl == "" || docksProviderUrl == "-" {
		return
	}
	adpClient := client.New(http.DefaultClient, docksProviderUrl)
	file, err := os.Open("docs/asyncapi.json")
	if err != nil {
		log.Println(err)
		return
	}
	defer file.Close()
	ctx, cf := context.WithTimeout(context.Background(), 30*time.Second)
	defer cf()
	err = adpClient.AsyncapiPutDocFromReader(ctx, "github_com_SENERGY-Platform_analytics-serving", file)
	if err != nil {
		log.Println(err)
		return
	}
}
