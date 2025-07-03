package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tsukinoko-kun/pogo/client"
	"github.com/tsukinoko-kun/pogo/colors"
)

var (
	bookmarkCmd = &cobra.Command{
		Use:     "bookmark",
		Aliases: []string{"b"},
		Short:   "Manage bookmarks",
	}

	bookmarkListCmd = &cobra.Command{
		Use:     "list",
		Aliases: []string{"l"},
		Short:   "List bookmarks and their targets",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.Open(".pogo")
			if err != nil {
				return fmt.Errorf("open repository: %w", err)
			}

			if err := c.Push(); err != nil {
				return errors.Join(errors.New("push"), err)
			}

			bookmarks, err := c.ListBookmarks()
			if err != nil {
				return fmt.Errorf("list bookmarks: %w", err)
			}

			longestBookmarkName := 0
			for _, bookmark := range bookmarks {
				if len(bookmark.BookmarkName) > longestBookmarkName {
					longestBookmarkName = len(bookmark.BookmarkName)
				}
			}

			for _, bookmark := range bookmarks {
				fmt.Println(
					strings.Repeat(" ", longestBookmarkName-len(bookmark.BookmarkName)) + bookmark.BookmarkName +
						" → " +
						colors.Magenta + bookmark.ChangePrefix + colors.BrightBlack + bookmark.ChangeName[len(bookmark.ChangePrefix):] + colors.Reset,
				)
			}

			return nil
		},
	}
)

func init() {
	bookmarkCmd.AddCommand(bookmarkListCmd)
	RootCmd.AddCommand(bookmarkCmd)
}
