package requests

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"

	"github.com/calmcacil/CalmsToolkit/internal/colors"
)

func testConnection(ctx context.Context, cfg ToolConfig) error {
	req, err := http.NewRequestWithContext(ctx, "GET", cfg.ServerURL+"/api/v1/auth/me", nil)
	if err != nil {
		return err
	}

	req.Header.Set("X-Api-Key", cfg.APIKey)
	client := &http.Client{Timeout: cfg.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("invalid API key")
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	return nil
}

func runInteractiveMenu(ctx context.Context, cfg ToolConfig) {
	reader := bufio.NewReader(os.Stdin)

	for {
		clearScreen()
		printMainMenu(cfg)

		input, _ := readKeystroke(cfg)

		switch input {
		case "n":
			handleNewRequest(ctx, cfg, reader)
		case "w":
			handleViewRequests(ctx, cfg, reader)
		case "q":
			fmt.Println("\nGoodbye!")
			return
		default:
			fmt.Println("\nInvalid option. Press any key to continue...")
			readKeystroke(cfg)
		}
	}
}

func printMainMenu(cfg ToolConfig) {
	p := colors.GetPalette(cfg.Theme)
	clr := colors.ClrFunc(cfg.NoColor)

	fmt.Printf("%s%s╔══════════════════════════════════════════╗%s\n", clr(p.Bold), clr(p.Accent), clr(p.Reset))
	fmt.Printf("%s%s║    Media Requests - Interactive Menu    ║%s\n", clr(p.Bold), clr(p.Accent), clr(p.Reset))
	fmt.Printf("%s%s╚══════════════════════════════════════════╝%s\n\n", clr(p.Bold), clr(p.Accent), clr(p.Reset))

	fmt.Printf("%s[N]%s New Request\n", clr(p.Success), clr(p.Reset))
	fmt.Printf("%s[W]%s View Requests\n", clr(p.Warning), clr(p.Reset))
	fmt.Printf("%s[Q]%s Quit\n\n", clr(p.Error), clr(p.Reset))
	fmt.Printf("Select an option: ")
}

func handleNewRequest(ctx context.Context, cfg ToolConfig, reader *bufio.Reader) {
	p := colors.GetPalette(cfg.Theme)
	clearScreen()
	clr := colors.ClrFunc(cfg.NoColor)

	fmt.Printf("%s%s=== New Media Request ===%s\n\n", clr(p.Bold), clr(p.Accent), clr(p.Reset))
	fmt.Printf("Enter search query (or 'back' to return): ")

	query, _ := reader.ReadString('\n')
	query = strings.TrimSpace(query)

	if query == "" || strings.ToLower(query) == "back" {
		return
	}

	fmt.Fprintf(os.Stderr, "\n%sSearching...%s\n", clr(p.Warning), clr(p.Reset))
	results, err := searchMedia(ctx, cfg, query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n%sError searching: %v%s\n", clr(p.Error), err, clr(p.Reset))
		fmt.Printf("\nPress any key to continue...")
		readKeystroke(cfg)
		return
	}

	if len(results) == 0 {
		fmt.Fprintf(os.Stderr, "\n%sNo results found.%s\n", clr(p.Warning), clr(p.Reset))
		fmt.Printf("\nPress any key to continue...")
		readKeystroke(cfg)
		return
	}

	clearScreen()
	fmt.Printf("%s%s=== Search Results ===%s\n\n", clr(p.Bold), clr(p.Accent), clr(p.Reset))

	displayLimit := 10
	if len(results) > displayLimit {
		results = results[:displayLimit]
	}

	for i, result := range results {
		displaySearchResult(cfg, i+1, result)
	}

	fmt.Printf("\nSelect a number (1-%d) or 'back' to cancel: ", len(results))
	selectionStr, _ := reader.ReadString('\n')
	selectionStr = strings.TrimSpace(selectionStr)

	if selectionStr == "" || strings.ToLower(selectionStr) == "back" {
		return
	}

	selection, err := strconv.Atoi(selectionStr)
	if err != nil || selection < 1 || selection > len(results) {
		fmt.Fprintf(os.Stderr, "\n%sInvalid selection.%s\n", clr(p.Error), clr(p.Reset))
		fmt.Printf("\nPress any key to continue...")
		readKeystroke(cfg)
		return
	}

	selectedMedia := results[selection-1]

	if selectedMedia.MediaInfo != nil {
		status := selectedMedia.MediaInfo.Status
		if status == MediaStatusAvailable || status == MediaStatusPartiallyAvailable {
			fmt.Fprintf(os.Stderr, "\n%sThis media is already available!%s\n", clr(p.Success), clr(p.Reset))
			fmt.Printf("\nPress any key to continue...")
			readKeystroke(cfg)
			return
		}
		if len(selectedMedia.MediaInfo.Requests) > 0 {
			fmt.Fprintf(os.Stderr, "\n%sThis media has already been requested.%s\n", clr(p.Warning), clr(p.Reset))
			fmt.Printf("\nPress any key to continue...")
			readKeystroke(cfg)
			return
		}
	}

	var seasons interface{}
	if selectedMedia.MediaType == "tv" {
		seasons, err = selectSeasons(ctx, cfg, selectedMedia, reader)
		if err != nil || seasons == nil {
			return
		}
	}

	overrides, err := selectRootFolderOverride(ctx, cfg, selectedMedia, reader)
	if err != nil {
		return
	}

	clearScreen()
	fmt.Printf("%s%s=== Confirm Request ===%s\n\n", clr(p.Bold), clr(p.Accent), clr(p.Reset))

	title := selectedMedia.Title
	if title == "" {
		title = selectedMedia.Name
	}
	year := getYear(selectedMedia)

	fmt.Printf("%sMedia:%s %s", clr(p.Bold), clr(p.Reset), title)
	if year != "" {
		fmt.Printf(" %s(%s)%s", clr(p.Accent), year, clr(p.Reset))
	}
	fmt.Printf("\n")

	fmt.Printf("%sType:%s %s\n", clr(p.Bold), clr(p.Reset), titleCase(selectedMedia.MediaType))

	if selectedMedia.MediaType == "tv" && seasons != nil {
		if seasons == "all" {
			fmt.Printf("%sSeasons:%s All\n", clr(p.Bold), clr(p.Reset))
		} else if seasonList, ok := seasons.([]int); ok {
			fmt.Printf("%sSeasons:%s %v\n", clr(p.Bold), clr(p.Reset), seasonList)
		}
	}

	if overrides != nil {
		if overrides.ServerName != "" {
			fmt.Printf("%sServer:%s %s\n", clr(p.Bold), clr(p.Reset), overrides.ServerName)
		}
		if overrides.RootFolder != "" {
			fmt.Printf("%sRoot Folder:%s %s\n", clr(p.Bold), clr(p.Reset), overrides.RootFolder)
		}
	}

	fmt.Printf("\nSubmit request? (y/n): ")
	confirm := readKeyOrDefault(cfg, "n")

	if confirm != "y" {
		fmt.Fprintf(os.Stderr, "\n%sRequest cancelled.%s\n", clr(p.Warning), clr(p.Reset))
		fmt.Printf("\nPress any key to continue...")
		readKeystroke(cfg)
		return
	}

	fmt.Fprintf(os.Stderr, "\n%sSubmitting request...%s\n", clr(p.Warning), clr(p.Reset))
	request, err := createRequest(ctx, cfg, selectedMedia, seasons, overrides)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n%sError creating request: %v%s\n", clr(p.Error), err, clr(p.Reset))
		fmt.Printf("\nPress any key to continue...")
		readKeystroke(cfg)
		return
	}

	fmt.Fprintf(os.Stderr, "\n%s✓ Request submitted successfully!%s\n", clr(p.Success), clr(p.Reset))
	fmt.Fprintf(os.Stderr, "Request ID: %s%d%s\n", clr(p.Accent), request.ID, clr(p.Reset))

	statusText := getStatusText(request.Status)
	fmt.Fprintf(os.Stderr, "Status: %s%s%s\n", clr(p.Warning), statusText, clr(p.Reset))

	fmt.Printf("\nPress any key to continue...")
	readKeystroke(cfg)
}

func handleViewRequests(ctx context.Context, cfg ToolConfig, reader *bufio.Reader) {
	p := colors.GetPalette(cfg.Theme)
	clearScreen()
	clr := colors.ClrFunc(cfg.NoColor)

	fmt.Printf("%s%s=== Pending Requests ===%s\n\n", clr(p.Bold), clr(p.Accent), clr(p.Reset))
	fmt.Fprintf(os.Stderr, "%sLoading...%s\n", clr(p.Warning), clr(p.Reset))

	requests, err := getPendingRequests(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n%sError fetching requests: %v%s\n", clr(p.Error), err, clr(p.Reset))
		fmt.Printf("\nPress any key to continue...")
		readKeystroke(cfg)
		return
	}

	clearScreen()
	fmt.Printf("%s%s=== Pending Requests ===%s\n\n", clr(p.Bold), clr(p.Accent), clr(p.Reset))

	if len(requests) == 0 {
		fmt.Printf("%sNo pending requests.%s\n", clr(p.Success), clr(p.Reset))
		fmt.Printf("\nPress any key to continue...")
		readKeystroke(cfg)
		return
	}

	for i, req := range requests {
		displayRequestSummary(cfg, i+1, req)
	}

	fmt.Printf("\nSelect a request (1-%d), or 'back' to return: ", len(requests))
	selectionStr, _ := reader.ReadString('\n')
	selectionStr = strings.TrimSpace(selectionStr)

	if selectionStr == "" || strings.ToLower(selectionStr) == "back" {
		return
	}

	selection, err := strconv.Atoi(selectionStr)
	if err != nil || selection < 1 || selection > len(requests) {
		fmt.Fprintf(os.Stderr, "\n%sInvalid selection.%s\n", clr(p.Error), clr(p.Reset))
		fmt.Printf("\nPress any key to continue...")
		readKeystroke(cfg)
		return
	}

	selectedRequest := requests[selection-1]
	handleRequestDetail(ctx, cfg, selectedRequest, reader)
}

func handleRequestDetail(ctx context.Context, cfg ToolConfig, request MediaRequest, reader *bufio.Reader) {
	p := colors.GetPalette(cfg.Theme)
	clearScreen()
	clr := colors.ClrFunc(cfg.NoColor)

	fmt.Printf("%s%s=== Request Details ===%s\n\n", clr(p.Bold), clr(p.Accent), clr(p.Reset))

	displayRequestDetail(cfg, request)

	fmt.Printf("\n%sActions:%s\n", clr(p.Bold), clr(p.Reset))
	fmt.Printf("%s[A]%s Approve    %s[D]%s Decline    %s[B]%s Back\n\n",
		clr(p.Success), clr(p.Reset),
		clr(p.Error), clr(p.Reset),
		clr(p.Warning), clr(p.Reset))

	fmt.Printf("Select action: ")
	action := readKeyOrDefault(cfg, "b")

	switch action {
	case "a":
		overrides, err := selectRootFolderForApproval(ctx, cfg, request, reader)
		if err != nil {
			return
		}

		fmt.Fprintf(os.Stderr, "\n%sApproving request...%s\n", clr(p.Warning), clr(p.Reset))
		if err := approveRequestWithOverrides(ctx, cfg, request.ID, overrides); err != nil {
			fmt.Fprintf(os.Stderr, "\n%sError approving: %v%s\n", clr(p.Error), err, clr(p.Reset))
		} else {
			fmt.Fprintf(os.Stderr, "\n%s✓ Request approved!%s\n", clr(p.Success), clr(p.Reset))
			if overrides != nil && overrides.RootFolder != "" {
				fmt.Fprintf(os.Stderr, "%s  Root folder set to: %s%s\n", clr(p.Subdued), overrides.RootFolder, clr(p.Reset))
			}
		}
		fmt.Printf("\nPress any key to continue...")
		readKeystroke(cfg)

	case "d":
		fmt.Printf("\n%sAre you sure you want to decline this request? (y/n):%s ", clr(p.Error), clr(p.Reset))
		confirm := readKeyOrDefault(cfg, "n")

		if confirm == "y" {
			fmt.Fprintf(os.Stderr, "\n%sDeclining request...%s\n", clr(p.Warning), clr(p.Reset))
			if err := declineRequest(ctx, cfg, request.ID); err != nil {
				fmt.Fprintf(os.Stderr, "\n%sError declining: %v%s\n", clr(p.Error), err, clr(p.Reset))
			} else {
				fmt.Fprintf(os.Stderr, "\n%s✓ Request declined.%s\n", clr(p.Success), clr(p.Reset))
			}
		} else {
			fmt.Fprintf(os.Stderr, "\n%sCancelled.%s\n", clr(p.Warning), clr(p.Reset))
		}
		fmt.Printf("\nPress any key to continue...")
		readKeystroke(cfg)

	case "b", "":
		return

	default:
		fmt.Fprintf(os.Stderr, "\n%sInvalid action.%s\n", clr(p.Error), clr(p.Reset))
		fmt.Printf("\nPress any key to continue...")
		readKeystroke(cfg)
	}
}

func selectSeasons(ctx context.Context, cfg ToolConfig, media SearchResult, reader *bufio.Reader) (interface{}, error) {
	p := colors.GetPalette(cfg.Theme)
	clr := colors.ClrFunc(cfg.NoColor)

	details, err := getTVDetails(ctx, cfg, media.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n%sError fetching TV show details: %v%s\n", clr(p.Error), err, clr(p.Reset))
		fmt.Printf("\nPress any key to continue...")
		readKeystroke(cfg)
		return nil, err
	}

	clearScreen()
	fmt.Printf("%s%s=== Select Seasons ===%s\n\n", clr(p.Bold), clr(p.Accent), clr(p.Reset))

	title := media.Title
	if title == "" {
		title = media.Name
	}
	fmt.Printf("%sTV Show:%s %s\n", clr(p.Bold), clr(p.Reset), title)
	fmt.Printf("%sTotal Seasons:%s %d\n\n", clr(p.Bold), clr(p.Reset), details.NumberOfSeasons)

	fmt.Printf("%s[A]%s Request all seasons\n", clr(p.Success), clr(p.Reset))
	fmt.Printf("%s[S]%s Select specific seasons\n", clr(p.Warning), clr(p.Reset))
	fmt.Printf("%s[B]%s Back\n\n", clr(p.Error), clr(p.Reset))

	fmt.Printf("Select option: ")
	option := readKeyOrDefault(cfg, "b")

	switch option {
	case "a":
		return "all", nil

	case "s":
		fmt.Printf("\nEnter season numbers (comma-separated, e.g., 1,2,3): ")
		seasonsStr, _ := reader.ReadString('\n')
		seasonsStr = strings.TrimSpace(seasonsStr)

		if seasonsStr == "" {
			return nil, fmt.Errorf("no seasons specified")
		}

		parts := strings.Split(seasonsStr, ",")
		seasons := make([]int, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			season, err := strconv.Atoi(part)
			if err != nil || season < 1 || season > details.NumberOfSeasons {
				fmt.Fprintf(os.Stderr, "\n%sInvalid season number: %s%s\n", clr(p.Error), part, clr(p.Reset))
				fmt.Printf("\nPress any key to continue...")
				readKeystroke(cfg)
				return nil, fmt.Errorf("invalid season number")
			}
			seasons = append(seasons, season)
		}

		if len(seasons) == 0 {
			return nil, fmt.Errorf("no valid seasons specified")
		}

		return seasons, nil

	case "b", "":
		return nil, fmt.Errorf("cancelled")

	default:
		fmt.Fprintf(os.Stderr, "\n%sInvalid option.%s\n", clr(p.Error), clr(p.Reset))
		fmt.Printf("\nPress any key to continue...")
		readKeystroke(cfg)
		return nil, fmt.Errorf("invalid option")
	}
}

func selectRootFolderOverride(ctx context.Context, cfg ToolConfig, media SearchResult, reader *bufio.Reader) (*RequestOverrides, error) {
	p := colors.GetPalette(cfg.Theme)
	mediaType := strings.ToLower(media.MediaType)

	var service string
	var serviceLabel string
	switch mediaType {
	case "movie":
		service = "radarr"
		serviceLabel = "Radarr"
	case "tv":
		service = "sonarr"
		serviceLabel = "Sonarr"
	default:
		return nil, nil
	}

	servers, err := fetchServiceInstances(ctx, cfg, service)
	if err != nil {
		clr := colors.ClrFunc(cfg.NoColor)
		fmt.Fprintf(os.Stderr, "\n%sError fetching %s servers: %v%s\n", clr(p.Error), serviceLabel, err, clr(p.Reset))
		fmt.Printf("\nPress any key to continue...")
		readKeystroke(cfg)
		return nil, err
	}

	if len(servers) == 0 {
		return nil, nil
	}

	clr := colors.ClrFunc(cfg.NoColor)

	fmt.Printf("\n%sSelect %s destination%s\n", clr(p.Bold), serviceLabel, clr(p.Reset))

	var selected *ServiceInstance

	if len(servers) > 1 {
		for {
			fmt.Printf("\nAvailable %s servers:\n", serviceLabel)
			for i, server := range servers {
				fmt.Printf("%s%d.%s %s", clr(p.Warning), i+1, clr(p.Reset), server.Name)

				var badges []string
				if server.IsDefault {
					badges = append(badges, "default")
				}
				if server.Is4k {
					badges = append(badges, "4K")
				}
				if len(badges) > 0 {
					fmt.Printf(" %s[%s]%s", clr(p.Subdued), strings.Join(badges, ", "), clr(p.Reset))
				}
				fmt.Println()
			}

			fmt.Printf("\nSelect a server (1-%d), press Enter to use defaults, or type 'back' to cancel: ", len(servers))
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(strings.ToLower(input))

			switch input {
			case "":
				for i := range servers {
					if servers[i].IsDefault {
						selected = &servers[i]
						break
					}
				}
				if selected == nil {
					selected = &servers[0]
				}
				fmt.Fprintf(os.Stderr, "Using default %s server: %s%s%s\n", serviceLabel, clr(p.Bold), selected.Name, clr(p.Reset))
			case "back", "b":
				return nil, fmt.Errorf("cancelled")
			default:
				index, convErr := strconv.Atoi(input)
				if convErr != nil || index < 1 || index > len(servers) {
					fmt.Fprintf(os.Stderr, "\n%sInvalid selection.%s\n", clr(p.Error), clr(p.Reset))
					continue
				}
				selected = &servers[index-1]
			}

			if selected != nil {
				break
			}
		}
	} else {
		selected = &servers[0]
		fmt.Fprintf(os.Stderr, "Using %s server: %s%s%s\n", serviceLabel, clr(p.Bold), selected.Name, clr(p.Reset))
	}

	details, err := fetchServiceDetails(ctx, cfg, service, selected.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n%sError fetching %s details: %v%s\n", clr(p.Error), serviceLabel, err, clr(p.Reset))
		fmt.Printf("\nPress any key to continue...")
		readKeystroke(cfg)
		return nil, err
	}

	if len(details.RootFolders) == 0 {
		fmt.Fprintf(os.Stderr, "\n%sNo root folders configured for %s.%s\n", clr(p.Warning), selected.Name, clr(p.Reset))
		fmt.Printf("Press Enter to continue...")
		reader.ReadString('\n')
		return &RequestOverrides{ServerID: selected.ID, ServerName: selected.Name}, nil
	}

	for {
		fmt.Printf("\n%sRoot folders for %s:%s\n", clr(p.Bold), selected.Name, clr(p.Reset))
		for i, folder := range details.RootFolders {
			fmt.Printf("%s%d.%s %s\n", clr(p.Warning), i+1, clr(p.Reset), folder.Path)
		}

		fmt.Printf("\nSelect a root folder (1-%d), press Enter to use server default, or type 'back' to cancel: ", len(details.RootFolders))
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))

		switch input {
		case "":
			return &RequestOverrides{ServerID: selected.ID, ServerName: selected.Name}, nil
		case "back", "b":
			return nil, fmt.Errorf("cancelled")
		default:
			index, convErr := strconv.Atoi(input)
			if convErr != nil || index < 1 || index > len(details.RootFolders) {
				fmt.Fprintf(os.Stderr, "\n%sInvalid selection.%s\n", clr(p.Error), clr(p.Reset))
				continue
			}
			folder := details.RootFolders[index-1]
			return &RequestOverrides{
				ServerID:   selected.ID,
				ServerName: selected.Name,
				RootFolder: folder.Path,
			}, nil
		}
	}
}

func readKeystroke(cfg ToolConfig) (string, error) {
	fd := int(os.Stdin.Fd())

	if !term.IsTerminal(fd) {
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(strings.ToLower(input)), nil
	}

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(strings.ToLower(input)), nil
	}

	defer term.Restore(fd, oldState)

	b := make([]byte, 1)
	_, err = os.Stdin.Read(b)
	if err != nil {
		return "", err
	}

	fmt.Printf("%c\n", b[0])

	char := strings.ToLower(string(b[0]))
	return char, nil
}

func readKeyOrDefault(cfg ToolConfig, defaultKey string) string {
	key, err := readKeystroke(cfg)
	if err != nil {
		return defaultKey
	}

	if key == "\n" || key == "\r" {
		return defaultKey
	}

	return key
}

func clearScreen() {
	fmt.Print(colors.ClearScreen + colors.HomeCursor)
}

func selectRootFolderForApproval(ctx context.Context, cfg ToolConfig, request MediaRequest, reader *bufio.Reader) (*RequestOverrides, error) {
	p := colors.GetPalette(cfg.Theme)
	clr := colors.ClrFunc(cfg.NoColor)

	var service string
	var serviceLabel string
	switch request.Type {
	case "movie":
		service = "radarr"
		serviceLabel = "Radarr"
	case "tv":
		service = "sonarr"
		serviceLabel = "Sonarr"
	default:
		return nil, nil
	}

	servers, err := fetchServiceInstances(ctx, cfg, service)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n%sError fetching %s servers: %v%s\n", clr(p.Error), serviceLabel, err, clr(p.Reset))
		fmt.Fprintf(os.Stderr, "Proceeding with approval without overrides...\n")
		return nil, nil
	}

	if len(servers) == 0 {
		return nil, nil
	}

	clearScreen()
	fmt.Printf("%s%s=== Approve Request - Root Folder Override ===%s\n\n", clr(p.Bold), clr(p.Accent), clr(p.Reset))

	displayRequestDetail(cfg, request)

	if request.RootFolder != "" {
		fmt.Printf("\n%sCurrent Root Folder:%s %s%s%s\n",
			clr(p.Bold), clr(p.Reset),
			clr(p.Accent), request.RootFolder, clr(p.Reset))
	} else {
		fmt.Printf("\n%sCurrent Root Folder:%s %sNot set (will use server default)%s\n",
			clr(p.Bold), clr(p.Reset),
			clr(p.Subdued), clr(p.Reset))
	}

	fmt.Printf("\n%sWould you like to override the root folder for this request?%s\n", clr(p.Bold), clr(p.Reset))
	fmt.Printf("%s[Y]%s Yes, select root folder\n", clr(p.Success), clr(p.Reset))
	fmt.Printf("%s[N]%s No, use default (proceed with approval)\n", clr(p.Warning), clr(p.Reset))
	fmt.Printf("%s[B]%s Back (cancel approval)\n\n", clr(p.Error), clr(p.Reset))

	fmt.Printf("Select option: ")
	option := readKeyOrDefault(cfg, "n")

	switch option {
	case "n", "":
		return nil, nil

	case "b", "back":
		return nil, fmt.Errorf("cancelled")

	case "y", "yes":
		break

	default:
		fmt.Fprintf(os.Stderr, "\n%sInvalid option.%s\n", clr(p.Error), clr(p.Reset))
		fmt.Printf("\nPress any key to continue...")
		readKeystroke(cfg)
		return nil, fmt.Errorf("invalid option")
	}

	var selected *ServiceInstance

	if len(servers) > 1 {
		for {
			clearScreen()
			fmt.Printf("%s%s=== Select %s Server ===%s\n\n", clr(p.Bold), clr(p.Accent), serviceLabel, clr(p.Reset))

			fmt.Printf("Available %s servers:\n", serviceLabel)
			for i, server := range servers {
				fmt.Printf("%s%d.%s %s", clr(p.Warning), i+1, clr(p.Reset), server.Name)

				var badges []string
				if server.IsDefault {
					badges = append(badges, "default")
				}
				if server.Is4k {
					badges = append(badges, "4K")
				}
				if len(badges) > 0 {
					fmt.Printf(" %s[%s]%s", clr(p.Subdued), strings.Join(badges, ", "), clr(p.Reset))
				}
				fmt.Println()
			}

			fmt.Printf("\nSelect a server (1-%d) or type 'back' to cancel: ", len(servers))
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(strings.ToLower(input))

			switch input {
			case "back", "b", "":
				return nil, fmt.Errorf("cancelled")
			default:
				index, convErr := strconv.Atoi(input)
				if convErr != nil || index < 1 || index > len(servers) {
					fmt.Fprintf(os.Stderr, "\n%sInvalid selection.%s\n", clr(p.Error), clr(p.Reset))
					fmt.Printf("\nPress Enter to continue...")
					reader.ReadString('\n')
					continue
				}
				selected = &servers[index-1]
			}

			if selected != nil {
				break
			}
		}
	} else {
		selected = &servers[0]
		fmt.Fprintf(os.Stderr, "\nUsing %s server: %s%s%s\n", serviceLabel, clr(p.Bold), selected.Name, clr(p.Reset))
	}

	details, err := fetchServiceDetails(ctx, cfg, service, selected.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n%sError fetching %s details: %v%s\n", clr(p.Error), serviceLabel, err, clr(p.Reset))
		fmt.Fprintf(os.Stderr, "Proceeding with approval without overrides...\n")
		fmt.Printf("\nPress any key to continue...")
		readKeystroke(cfg)
		return nil, nil
	}

	if len(details.RootFolders) == 0 {
		fmt.Fprintf(os.Stderr, "\n%sNo root folders configured for %s.%s\n", clr(p.Warning), selected.Name, clr(p.Reset))
		fmt.Fprintf(os.Stderr, "Proceeding with approval without overrides...\n")
		fmt.Printf("\nPress any key to continue...")
		readKeystroke(cfg)
		return &RequestOverrides{ServerID: selected.ID, ServerName: selected.Name}, nil
	}

	for {
		clearScreen()
		fmt.Printf("%s%s=== Select Root Folder ===%s\n\n", clr(p.Bold), clr(p.Accent), clr(p.Reset))

		fmt.Printf("%sServer:%s %s\n\n", clr(p.Bold), clr(p.Reset), selected.Name)
		fmt.Printf("%sRoot folders:%s\n", clr(p.Bold), clr(p.Reset))
		for i, folder := range details.RootFolders {
			fmt.Printf("%s%d.%s %s\n", clr(p.Warning), i+1, clr(p.Reset), folder.Path)
		}

		fmt.Printf("\nSelect a root folder (1-%d) or type 'back' to cancel: ", len(details.RootFolders))
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))

		switch input {
		case "back", "b", "":
			return nil, fmt.Errorf("cancelled")
		default:
			index, convErr := strconv.Atoi(input)
			if convErr != nil || index < 1 || index > len(details.RootFolders) {
				fmt.Fprintf(os.Stderr, "\n%sInvalid selection.%s\n", clr(p.Error), clr(p.Reset))
				fmt.Printf("\nPress Enter to continue...")
				reader.ReadString('\n')
				continue
			}
			folder := details.RootFolders[index-1]
			return &RequestOverrides{
				ServerID:   selected.ID,
				ServerName: selected.Name,
				RootFolder: folder.Path,
			}, nil
		}
	}
}
