package requests

import (
	"fmt"

	"github.com/calmcacil/CalmsToolkit/internal/colors"
)

func displaySearchResult(cfg ToolConfig, index int, result SearchResult) {
	p := colors.GetPalette(cfg.Theme)
	clr := colors.ClrFunc(cfg.NoColor)

	title := result.Title
	if title == "" {
		title = result.Name
	}

	year := getYear(result)

	typeIcon := "[MOVIE]"
	if result.MediaType == "tv" {
		typeIcon = "[TV]"
	}

	fmt.Printf("%s%d.%s %s %s%s%s",
		clr(p.Warning), index, clr(p.Reset),
		typeIcon, clr(p.Bold), title, clr(p.Reset))

	if year != "" {
		fmt.Printf(" %s(%s)%s", clr(p.Accent), year, clr(p.Reset))
	}

	if result.MediaInfo != nil {
		status := result.MediaInfo.Status
		if status == MediaStatusAvailable || status == MediaStatusPartiallyAvailable {
			fmt.Printf(" %s[AVAILABLE]%s", clr(p.Success), clr(p.Reset))
		} else if len(result.MediaInfo.Requests) > 0 {
			fmt.Printf(" %s[REQUESTED]%s", clr(p.Warning), clr(p.Reset))
		}
	}

	fmt.Printf("\n")

	if result.Overview != "" && len(result.Overview) > 100 {
		fmt.Printf("   %s%s...%s\n", clr(p.Subdued), result.Overview[:97], clr(p.Reset))
	} else if result.Overview != "" {
		fmt.Printf("   %s%s%s\n", clr(p.Subdued), result.Overview, clr(p.Reset))
	}

	if result.VoteAverage > 0 {
		fmt.Printf("   %sRating: %.1f/10%s\n", clr(p.Subdued), result.VoteAverage, clr(p.Reset))
	}

	fmt.Println()
}

func displayRequestSummary(cfg ToolConfig, index int, request MediaRequest) {
	p := colors.GetPalette(cfg.Theme)
	clr := colors.ClrFunc(cfg.NoColor)

	mediaType := "Movie"
	if request.Type == "tv" {
		mediaType = "TV Show"
	}

	fmt.Printf("%s%d.%s [%s] ", clr(p.Warning), index, clr(p.Reset), mediaType)

	fmt.Printf("Request ID: %s%d%s ", clr(p.Accent), request.ID, clr(p.Reset))
	fmt.Printf("(TMDB: %d)", request.Media.TmdbID)

	requestedBy := request.RequestedBy.Username
	if requestedBy == "" {
		requestedBy = request.RequestedBy.Email
	}
	if requestedBy == "" {
		requestedBy = request.RequestedBy.DisplayName
	}

	fmt.Printf("\n   %sRequested by:%s %s", clr(p.Subdued), clr(p.Reset), requestedBy)
	fmt.Printf("  %sCreated:%s %s\n", clr(p.Subdued), clr(p.Reset), formatDate(request.CreatedAt))

	fmt.Println()
}

func displayRequestDetail(cfg ToolConfig, request MediaRequest) {
	p := colors.GetPalette(cfg.Theme)
	clr := colors.ClrFunc(cfg.NoColor)

	fmt.Printf("%sRequest ID:%s %d\n", clr(p.Bold), clr(p.Reset), request.ID)
	fmt.Printf("%sTMDB ID:%s %d\n", clr(p.Bold), clr(p.Reset), request.Media.TmdbID)

	mediaType := "Movie"
	if request.Type == "tv" {
		mediaType = "TV Show"
	}
	fmt.Printf("%sType:%s %s\n", clr(p.Bold), clr(p.Reset), mediaType)

	requestedBy := request.RequestedBy.Username
	if requestedBy == "" {
		requestedBy = request.RequestedBy.Email
	}
	if requestedBy == "" {
		requestedBy = request.RequestedBy.DisplayName
	}
	fmt.Printf("%sRequested by:%s %s\n", clr(p.Bold), clr(p.Reset), requestedBy)

	fmt.Printf("%sCreated:%s %s\n", clr(p.Bold), clr(p.Reset), formatDate(request.CreatedAt))
	fmt.Printf("%sStatus:%s %s%s%s\n", clr(p.Bold), clr(p.Reset),
		clr(p.Warning), getStatusText(request.Status), clr(p.Reset))

	if len(request.Seasons) > 0 {
		fmt.Printf("%sSeasons requested:%s %d\n", clr(p.Bold), clr(p.Reset), len(request.Seasons))
	}

	if request.Is4k {
		fmt.Printf("%s4K:%s Yes\n", clr(p.Bold), clr(p.Reset))
	}
}
