package api_doc

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/SENERGY-Platform/analytics-serving/pkg/util"
	"github.com/SENERGY-Platform/api-docs-provider/lib/client"
)

func PublishAsyncapiDoc(docksProviderUrl string) {
	if docksProviderUrl == "" || docksProviderUrl == "-" {
		return
	}
	adpClient := client.New(http.DefaultClient, docksProviderUrl)
	file, err := os.Open("docs/asyncapi.json")
	if err != nil {
		util.Logger.Error("error opening asyncapi.json", "error", err)
		return
	}
	defer func(file *os.File) {
		err = file.Close()
		if err != nil {
			util.Logger.Error("error closing asyncapi.json", "error", err)
		}
	}(file)
	ctx, cf := context.WithTimeout(context.Background(), 30*time.Second)
	defer cf()
	err = adpClient.AsyncapiPutDocFromReader(ctx, "github_com_SENERGY-Platform_analytics-serving", file)
	if err != nil {
		util.Logger.Error("error AsyncapiPutDocFromReader", "error", err)
		return
	}
}
