package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/Architeg/gloss/internal/update"
)

type updateCheckerDownloader interface {
	Check(context.Context, string) (update.CheckResult, error)
	DownloadVerified(context.Context, update.Release) (update.VerifiedUpdate, error)
}

type inspectUpdateFunc func() (update.Layout, error)
type installUpdateFunc func(update.Layout, update.VerifiedUpdate) error

func runUpdateCLI(
	ctx context.Context,
	out io.Writer,
	install bool,
	client updateCheckerDownloader,
	inspect inspectUpdateFunc,
	replace installUpdateFunc,
) error {
	result, err := client.Check(ctx, Version)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Current version: %s\n", Version)
	fmt.Fprintf(out, "Latest stable version: %s\n", result.LatestVersion)
	if !result.UpdateAvailable {
		fmt.Fprintln(out, "Gloss is up to date.")
		return nil
	}
	fmt.Fprintf(out, "Update available: %s\n", result.LatestVersion)
	if !result.PlatformSupported {
		return fmt.Errorf("%w: this build cannot install release assets", update.ErrUnsupportedPlatform)
	}

	layout, layoutErr := inspect()
	if update.IsHomebrew(layoutErr) || layout.Kind == update.LayoutHomebrew {
		fmt.Fprintln(out, update.HomebrewUpgradeCommand)
		if install {
			return errors.New("self-update is disabled for Homebrew-managed installations")
		}
		return nil
	}
	if !install {
		if layoutErr != nil {
			fmt.Fprintf(out, "Self-install unavailable: %v\n", layoutErr)
		} else {
			fmt.Fprintln(out, "Run: gloss update --install")
		}
		return nil
	}
	if !result.CurrentValid {
		return fmt.Errorf("refusing to replace development or malformed version %q", Version)
	}
	if layoutErr != nil {
		return layoutErr
	}
	verified, err := client.DownloadVerified(ctx, result.Release)
	if err != nil {
		return err
	}
	if err := replace(layout, verified); err != nil {
		return err
	}
	fmt.Fprintf(out, "Installed Gloss %s.\n", verified.Version)
	fmt.Fprintln(out, "Rerun Gloss to use the new version.")
	return nil
}
