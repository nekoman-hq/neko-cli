package init

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      23.12.2025
*/

import (
	"fmt"

	"github.com/nekoman-hq/neko-cli/internal/config"
	"github.com/nekoman-hq/neko-cli/internal/log"
)

func printSetupInstructions(cfg config.NekoConfig) {
	log.Print(log.Init, fmt.Sprintf("%s .neko.json created successfully!\n",
		log.ColorText(log.ColorGreen, "\uF00C")))

	println(log.ColorText(log.ColorBold, "\nNext steps:"))
	println(fmt.Sprintf("  %s Use %s to create a release",
		log.ColorText(log.ColorCyan, "\uF101"),
		log.ColorText(log.ColorCyan, "'neko release'")))
	println(fmt.Sprintf("  %s Neko automatically manages the version in:",
		log.ColorText(log.ColorCyan, "\uF101")))

	switch cfg.ReleaseSystem {
	case config.ReleaseTypeReleaseIt:
		println("    package.json")
		println("    .release-it.json")
	case config.ReleaseTypeJReleaser:
		println("    jreleaser.yml")
		println("    pom.xml / build.gradle")
	case config.ReleaseTypeGoReleaser:
		println("    .goreleaser.yml")
		println("    Git tags")
	}

	println(fmt.Sprintf("\n%s The version in %s is the single source of truth.",
		log.ColorText(log.ColorCyan, "\uF0EB"),
		log.ColorText(log.ColorYellow, ".neko.json")))
}
