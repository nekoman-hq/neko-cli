package history

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      29.12.2025
*/

import (
	"fmt"

	"github.com/nekoman-hq/neko-cli/internal/git"
	"github.com/nekoman-hq/neko-cli/internal/log"
)

// ShowHistory displays the complete git history overview
func ShowHistory() {
	log.Print(log.History, "Starting git history overview")

	showBranch()
	showLastCommit()
	showStatistics()
	showTagHistory()
	showContributors()

	log.Print(log.History, "\uF00C Git history overview %s",
		log.ColorText(log.ColorGreen, "completed"))
}

// showBranch displays the current branch
func showBranch() {
	branch, err := git.CurrentBranch()
	if err != nil {
		return
	}

	fmt.Printf(" %s  %s \n",
		log.ColorText(log.ColorGreen, "\uE725"),
		branch,
	)
}

// showLastCommit displays the last commit information
func showLastCommit() {
	lastCommit, err := git.LastCommit()
	if err != nil {
		return
	}

	fmt.Printf(" %s  %s \n",
		log.ColorText(log.ColorYellow, "\uF172"),
		lastCommit,
	)
}

// showStatistics displays repository statistics
func showStatistics() {
	log.V(log.History, "Gathering repository statistics")

	// Total commits
	totalCommits, err := git.TotalCommits()
	if err != nil {
	}

	// Tags
	tagList := git.GetTags()
	// Files count
	filesCount, err := git.FilesCount()
	if err != nil {
		return
	}

	// Repo size
	repoSize, err := git.RepoSize()
	if err != nil {
		return
	}

	// Print statistics
	fmt.Println(log.ColorText(log.ColorCyan, "\n┌─ \uF201 Statistics"))
	fmt.Printf("%s  Commits:      %s\n",
		log.ColorText(log.ColorCyan, "│"),
		log.ColorText(log.ColorBlue, totalCommits),
	)
	fmt.Printf("%s  Tags:         %s\n",
		log.ColorText(log.ColorCyan, "│"),
		log.ColorText(log.ColorBlue, fmt.Sprintf("%d", len(tagList))),
	)
	fmt.Printf("%s  Files:        %s\n",
		log.ColorText(log.ColorCyan, "│"),
		log.ColorText(log.ColorBlue, fmt.Sprintf("%d", filesCount)),
	)
	if repoSize != "" {
		fmt.Printf("%s  Size:         %s\n",
			log.ColorText(log.ColorCyan, "│"),
			log.ColorText(log.ColorBlue, repoSize),
		)

	}
	fmt.Println(log.ColorText(log.ColorCyan, "│"))
}

// showTagHistory displays the tag history tree
func showTagHistory() {
	tagList := git.GetTags()
	if len(tagList) == 0 {
		log.V(log.History, "No tags found, skipping tag history")
		return
	}

	log.V(log.History, fmt.Sprintf("Building tag history tree (%d tags)", len(tagList)))

	fmt.Println(log.ColorText(log.ColorCyan, "├─ \U000F04F9 Tag History"))

	for i := range tagList {
		var commitCount int
		var prefix string

		if i == len(tagList)-1 {
			prefix = "└─"
		} else {
			prefix = "├─"
		}

		if i == 0 {
			commitCount = git.CountCommitsBetween("", tagList[i])
			fmt.Printf("%s %s %s (%s commits from start)\n",
				log.ColorText(log.ColorCyan, "│"),
				log.ColorText(log.ColorCyan, prefix),
				log.ColorText(log.ColorGreen, tagList[i]),
				log.ColorText(log.ColorBlue, fmt.Sprintf("%d", commitCount)),
			)
		} else {
			commitCount = git.CountCommitsBetween(tagList[i-1], tagList[i])
			fmt.Printf("%s %s %s → %s %s\n",
				log.ColorText(log.ColorCyan, "│"),
				log.ColorText(log.ColorCyan, prefix),
				log.ColorText(log.ColorGreen, tagList[i-1]),
				log.ColorText(log.ColorPurple, tagList[i]),
				log.ColorText(log.ColorBlue, fmt.Sprintf("+%d", commitCount)),
			)
		}
	}
	fmt.Println(log.ColorText(log.ColorCyan, "│"))

	log.V(log.History, "Tag history tree completed")
}

// showContributors displays repository contributors
func showContributors() {
	fmt.Println(log.ColorText(log.ColorCyan, "└─ \uF4FE Contributors"))

	contributors, err := git.Contributors()
	if err != nil {
		return
	}

	for i, contributor := range contributors {
		var prefix string
		if i == len(contributors)-1 {
			prefix = "   └─"
		} else {
			prefix = "   ├─"
		}

		fmt.Printf("%s %s commits: %s\n",
			log.ColorText(log.ColorCyan, prefix),
			log.ColorText(log.ColorBlue, contributor.Commits),
			contributor.Author,
		)
	}

	log.V(log.History, "\uF00C Contributors overview %s",
		log.ColorText(log.ColorGreen, "completed"))
}
