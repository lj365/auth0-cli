package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/auth0/auth0-cli/internal/ansi"
	"github.com/auth0/auth0-cli/internal/buildinfo"
)

const apiDocsURL = "https://auth0.com/docs/api/management/v2"

var apiFlags = apiCmdFlags{
	Data: Flag{
		Name:         "RawData",
		LongForm:     "data",
		ShortForm:    "d",
		Help:         "JSON data payload to send with the request.",
		IsRequired:   false,
		AlwaysPrompt: false,
	},
	QueryParams: Flag{
		Name:         "QueryParams",
		LongForm:     "query",
		ShortForm:    "q",
		Help:         "Query params to send with the request.",
		IsRequired:   false,
		AlwaysPrompt: false,
	},
}

var apiValidMethods = []string{
	http.MethodGet,
	http.MethodPost,
	http.MethodPut,
	http.MethodPatch,
	http.MethodDelete,
}

type (
	apiCmdFlags struct {
		Data        Flag
		QueryParams Flag
	}

	apiCmdInputs struct {
		RawMethod      string
		RawURI         string
		RawData        string
		RawQueryParams map[string]string
		Method         string
		URL            *url.URL
		Data           io.Reader
	}
)

func apiCmd(cli *cli) *cobra.Command {
	var inputs apiCmdInputs

	cmd := &cobra.Command{
		Use:   "api <method> <uri>",
		Args:  cobra.RangeArgs(1, 2),
		Short: "Makes an authenticated HTTP request to the Auth0 Management API",
		Long: fmt.Sprintf(
			`Makes an authenticated HTTP request to the Auth0 Management API and prints the response as JSON.

The method argument is optional, and when you don’t specify it, the command defaults to GET for requests without data and POST for requests with data.

%s  %s

%s  %s`,
			"Auth0 Management API Docs:\n", apiDocsURL,
			"Available Methods:\n", strings.Join(apiValidMethods, ", "),
		),
		Example: `auth0 api "/stats/daily" -q "from=20221101" -q "to=20221118"
auth0 api get "/tenants/settings"
auth0 api clients --data "{\"name\":\"ssoTest\",\"app_type\":\"sso_integration\"}"
`,
		RunE: apiCmdRun(cli, &inputs),
	}

	apiFlags.Data.RegisterString(cmd, &inputs.RawData, "")
	apiFlags.QueryParams.RegisterStringMap(cmd, &inputs.RawQueryParams, nil)

	return cmd
}

func apiCmdRun(cli *cli, inputs *apiCmdInputs) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if err := inputs.fromArgs(args, cli.tenant); err != nil {
			return fmt.Errorf("failed to parse command inputs: %w", err)
		}

		var response *http.Response
		if err := ansi.Waiting(func() error {
			request, err := http.NewRequestWithContext(
				cmd.Context(),
				inputs.Method,
				inputs.URL.String(),
				inputs.Data,
			)
			if err != nil {
				return err
			}

			bearerToken := cli.config.Tenants[cli.tenant].AccessToken
			request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", bearerToken))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("User-Agent", fmt.Sprintf("%s/%s", userAgent, strings.TrimPrefix(buildinfo.Version, "v")))

			response, err = http.DefaultClient.Do(request)
			return err
		}); err != nil {
			return fmt.Errorf("failed to send request: %w", err)
		}
		defer response.Body.Close()

		rawBodyJSON, err := io.ReadAll(response.Body)
		if err != nil {
			return err
		}

		var prettyJSON bytes.Buffer
		if err := json.Indent(&prettyJSON, rawBodyJSON, "", "  "); err != nil {
			return fmt.Errorf("failed to prepare json output: %w", err)
		}

		cli.renderer.Output(ansi.ColorizeJSON(prettyJSON.String(), false))

		return nil
	}
}

func (i *apiCmdInputs) fromArgs(args []string, domain string) error {
	i.parseRaw(args)

	if err := i.validateAndSetMethod(); err != nil {
		return err
	}

	if err := i.validateAndSetData(); err != nil {
		return err
	}

	return i.validateAndSetEndpoint(domain)
}

func (i *apiCmdInputs) validateAndSetMethod() error {
	for _, validMethod := range apiValidMethods {
		if i.RawMethod == validMethod {
			i.Method = i.RawMethod
			return nil
		}
	}

	return fmt.Errorf(
		"invalid method given: %s, accepting only %s",
		i.RawMethod,
		strings.Join(apiValidMethods, ", "),
	)
}

func (i *apiCmdInputs) validateAndSetData() error {
	if i.RawData != "" && !json.Valid([]byte(i.RawData)) {
		return fmt.Errorf("invalid json data given: %+v", i.RawData)
	}

	i.Data = bytes.NewReader([]byte(i.RawData))

	return nil
}

func (i *apiCmdInputs) validateAndSetEndpoint(domain string) error {
	endpoint, err := url.Parse("https://" + domain + "/api/v2/" + strings.Trim(i.RawURI, "/"))
	if err != nil {
		return fmt.Errorf("invalid uri given: %w", err)
	}

	params := endpoint.Query()
	for key, value := range i.RawQueryParams {
		params.Set(key, value)
	}
	endpoint.RawQuery = params.Encode()

	i.URL = endpoint

	return nil
}

func (i *apiCmdInputs) parseRaw(args []string) {
	lenArgs := len(args)
	if lenArgs == 1 {
		i.RawMethod = http.MethodGet
		if i.RawData != "" {
			i.RawMethod = http.MethodPost
		}
	} else {
		i.RawMethod = strings.ToUpper(args[0])
	}

	i.RawURI = args[lenArgs-1]
}
