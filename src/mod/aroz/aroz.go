package aroz

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
)

// To be used with arozos system
type ArozHandler struct {
	Port            string
	restfulEndpoint string
}

// Information required for registering this subservice to arozos
type ServiceInfo struct {
	Name         string   //Name of this module. e.g. "Audio"
	Desc         string   //Description for this module
	Group        string   //Group of the module, e.g. "system" / "media" etc
	IconPath     string   //Module icon image path e.g. "Audio/img/function_icon.png"
	Version      string   //Version of the module. Format: [0-9]*.[0-9][0-9].[0-9]
	StartDir     string   //Default starting dir, e.g. "Audio/index.html"
	SupportFW    bool     //Support floatWindow. If yes, floatWindow dir will be loaded
	LaunchFWDir  string   //This link will be launched instead of 'StartDir' if fw mode
	SupportEmb   bool     //Support embedded mode
	LaunchEmb    string   //This link will be launched instead of StartDir / Fw if a file is opened with this module
	InitFWSize   []int    //Floatwindow init size. [0] => Width, [1] => Height
	InitEmbSize  []int    //Embedded mode init size. [0] => Width, [1] => Height
	SupportedExt []string //Supported File Extensions. e.g. ".mp3", ".flac", ".wav"
}

// This function will request the required flag from the startup paramters and parse it to the need of the arozos.
func HandleFlagParse(info ServiceInfo) *ArozHandler {
	infoRequestMode := flag.Bool("info", false, "Show information about this program in JSON")
	port := flag.String("port", ":8000", "Management web interface listening port")
	restful := flag.String("rpt", "", "Reserved")
	//Parse the flags
	flag.Parse()
	if *infoRequestMode {
		//Information request mode
		jsonString, _ := json.MarshalIndent(info, "", " ")
		fmt.Println(string(jsonString))
		os.Exit(0)
	}
	return &ArozHandler{
		Port:            *port,
		restfulEndpoint: *restful,
	}
}

// Get the username and resources access token from the request, return username, token
func (a *ArozHandler) GetUserInfoFromRequest(w http.ResponseWriter, r *http.Request) (string, string) {
	return r.Header.Get("aouser"), r.Header.Get("aotoken")
}

func (a *ArozHandler) IsUsingExternalPermissionManager() bool {
	return !(a.restfulEndpoint == "")
}

// Request gateway interface for advance permission sandbox control
func (a *ArozHandler) RequestGatewayInterface(token string, script string) (*http.Response, error) {
	resp, err := http.PostForm(a.restfulEndpoint,
		url.Values{"token": {token}, "script": {script}})
	if err != nil {
		// handle error
		return nil, err
	}

	return resp, nil
}
