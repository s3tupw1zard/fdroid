package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-github/v39/github"
	"encoding/json"
	"net/http"
	"golang.org/x/oauth2"
	"metascoop/apps"
	"metascoop/file"
	"metascoop/git"
	"metascoop/md"
)

func handleGitHubRepo(client *github.Client, repo apps.Repo) error {
	log.Printf("Looking up %s/%s on GitHub", repo.Author, repo.Name)

	// Fetch repository details
	gitHubRepo, _, err := client.Repositories.Get(context.Background(), repo.Author, repo.Name)
	if err != nil {
		return fmt.Errorf("error accessing GitHub repository: %w", err)
	}

	// Log basic repository details
	log.Printf("Repository Name: %s", gitHubRepo.GetFullName())
	log.Printf("Description: %s", gitHubRepo.GetDescription())
	log.Printf("Stars: %d", gitHubRepo.GetStargazersCount())
	log.Printf("Forks: %d", gitHubRepo.GetForksCount())

	// Fetch latest release (if any)
	release, _, err := client.Repositories.GetLatestRelease(context.Background(), repo.Author, repo.Name)
	if err != nil {
		if _, ok := err.(*github.ErrorResponse); ok && err.(*github.ErrorResponse).Response.StatusCode == http.StatusNotFound {
			log.Printf("No releases found for %s/%s", repo.Author, repo.Name)
		} else {
			return fmt.Errorf("error fetching latest release: %w", err)
		}
	} else {
		log.Printf("Latest Release: %s", release.GetTagName())
		log.Printf("Release Name: %s", release.GetName())
		log.Printf("Published at: %s", release.GetPublishedAt().String())
		
		// Optionally download release assets
		for _, asset := range release.Assets {
			log.Printf("Asset: %s, Download Count: %d", asset.GetName(), asset.GetDownloadCount())
			// Download the asset if necessary (uncomment below if needed)
			// err := downloadAsset(asset)
			// if err != nil {
			//     return fmt.Errorf("error downloading asset: %w", err)
			// }
		}
	}

	// Optionally, clone the repository (uncomment below if needed)
	// cloneURL := gitHubRepo.GetCloneURL()
	// err = git.CloneRepo(cloneURL, localDir)
	// if err != nil {
	//     return fmt.Errorf("error cloning repository: %w", err)
	// }

	// Further processing (e.g., checking for APKs, etc.)

	return nil
}

// Uncomment and implement the downloadAsset function if you need to download assets
// func downloadAsset(asset *github.ReleaseAsset) error {
//     url := asset.GetBrowserDownloadURL()
//     fileName := asset.GetName()

//     // Create the file
//     out, err := os.Create(fileName)
//     if err != nil {
//         return err
//     }
//     defer out.Close()

//     // Get the data
//     resp, err := http.Get(url)
//     if err != nil {
//         return err
//     }
//     defer resp.Body.Close()

//     // Write the body to file
//     _, err = io.Copy(out, resp.Body)
//     return err
// }

func handleCodebergRepo(repo apps.Repo) error {
	log.Printf("Looking up %s/%s on Codeberg", repo.Author, repo.Name)

	// Fetch repository details from Codeberg API
	apiURL := fmt.Sprintf("https://codeberg.org/api/v1/repos/%s/%s", repo.Author, repo.Name)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code from Codeberg API: %d", resp.StatusCode)
	}

	var codebergRepo struct {
		FullName    string `json:"full_name"`
		Description string `json:"description"`
		Stars       int    `json:"stars_count"`
		Forks       int    `json:"forks_count"`
		CloneURL    string `json:"clone_url"`
	}

	err = json.NewDecoder(resp.Body).Decode(&codebergRepo)
	if err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Log basic repository details
	log.Printf("Repository Name: %s", codebergRepo.FullName)
	log.Printf("Description: %s", codebergRepo.Description)
	log.Printf("Stars: %d", codebergRepo.Stars)
	log.Printf("Forks: %d", codebergRepo.Forks)

	// Fetch releases (Codeberg uses a similar API structure to Gitea)
	releasesURL := fmt.Sprintf("https://codeberg.org/api/v1/repos/%s/%s/releases", repo.Author, repo.Name)
	req, err = http.NewRequest("GET", releasesURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request for releases: %w", err)
	}

	resp, err = client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute HTTP request for releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("No releases found for %s/%s on Codeberg", repo.Author, repo.Name)
	} else {
		var releases []struct {
			TagName    string `json:"tag_name"`
			Name       string `json:"name"`
			Published  string `json:"published_at"`
			Assets     []struct {
				Name        string `json:"name"`
				DownloadURL string `json:"browser_download_url"`
			} `json:"assets"`
		}

		err = json.NewDecoder(resp.Body).Decode(&releases)
		if err != nil {
			return fmt.Errorf("failed to decode releases: %w", err)
		}

		if len(releases) > 0 {
			latestRelease := releases[0]
			log.Printf("Latest Release: %s", latestRelease.TagName)
			log.Printf("Release Name: %s", latestRelease.Name)
			log.Printf("Published at: %s", latestRelease.Published)

			// Optionally download release assets
			for _, asset := range latestRelease.Assets {
				log.Printf("Asset: %s, Download URL: %s", asset.Name, asset.DownloadURL)
				// Download the asset if necessary (uncomment below if needed)
				// err := downloadAsset(asset.DownloadURL, asset.Name)
				// if err != nil {
				//     return fmt.Errorf("error downloading asset: %w", err)
				// }
			}
		} else {
			log.Printf("No releases found for %s/%s", repo.Author, repo.Name)
		}
	}

	// Optionally, clone the repository (uncomment below if needed)
	// err = git.CloneRepo(codebergRepo.CloneURL, localDir)
	// if err != nil {
	//     return fmt.Errorf("error cloning repository: %w", err)
	// }

	// Further processing (e.g., checking for APKs, etc.)

	return nil
}

// Uncomment and implement the downloadAsset function if you need to download assets
// func downloadAsset(downloadURL, fileName string) error {
//     // Create the file
//     out, err := os.Create(fileName)
//     if err != nil {
//         return err
//     }
//     defer out.Close()

//     // Get the data
//     resp, err := http.Get(downloadURL)
//     if err != nil {
//         return err
//     }
//     defer resp.Body.Close()

//     // Write the body to file
//     _, err = io.Copy(out, resp.Body)
//     return err
// }


func handleGitLabRepo(repo apps.Repo) error {
	log.Printf("Looking up %s/%s on GitLab", repo.Author, repo.Name)

	// Fetch repository details from GitLab API
	apiURL := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s%%2F%s", repo.Author, repo.Name)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code from GitLab API: %d", resp.StatusCode)
	}

	var gitLabRepo struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Stars       int    `json:"star_count"`
		Forks       int    `json:"forks_count"`
		CloneURL    string `json:"http_url_to_repo"`
	}

	err = json.NewDecoder(resp.Body).Decode(&gitLabRepo)
	if err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Log basic repository details
	log.Printf("Repository Name: %s", gitLabRepo.Name)
	log.Printf("Description: %s", gitLabRepo.Description)
	log.Printf("Stars: %d", gitLabRepo.Stars)
	log.Printf("Forks: %d", gitLabRepo.Forks)

	// Fetch tags (GitLab uses tags instead of traditional releases)
	tagsURL := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s%%2F%s/repository/tags", repo.Author, repo.Name)
	req, err = http.NewRequest("GET", tagsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request for tags: %w", err)
	}

	resp, err = client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute HTTP request for tags: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("No tags found for %s/%s on GitLab", repo.Author, repo.Name)
	} else {
		var tags []struct {
			Name       string `json:"name"`
			Message    string `json:"message"`
			Commit     struct {
				ID        string `json:"id"`
				CreatedAt string `json:"created_at"`
			} `json:"commit"`
			Release struct {
				TagName string `json:"tag_name"`
				Description string `json:"description"`
			} `json:"release"`
			TarballURL string `json:"tarball_url"`
		}

		err = json.NewDecoder(resp.Body).Decode(&tags)
		if err != nil {
			return fmt.Errorf("failed to decode tags: %w", err)
		}

		if len(tags) > 0 {
			latestTag := tags[0]
			log.Printf("Latest Tag: %s", latestTag.Name)
			log.Printf("Commit ID: %s", latestTag.Commit.ID)
			log.Printf("Created at: %s", latestTag.Commit.CreatedAt)

			if latestTag.Release.TagName != "" {
				log.Printf("Release Description: %s", latestTag.Release.Description)
			}

			// Optionally download the source code tarball for the latest tag
			log.Printf("Tarball URL: %s", latestTag.TarballURL)
			// Uncomment below to download the tarball
			// err := downloadTarball(latestTag.TarballURL, fmt.Sprintf("%s-%s.tar.gz", repo.Name, latestTag.Name))
			// if err != nil {
			//     return fmt.Errorf("error downloading tarball: %w", err)
			// }
		} else {
			log.Printf("No tags found for %s/%s", repo.Author, repo.Name)
		}
	}

	// Optionally, clone the repository (uncomment below if needed)
	// err = git.CloneRepo(gitLabRepo.CloneURL, localDir)
	// if err != nil {
		// return fmt.Errorf("error cloning repository: %w", err)
	// }

	// Further processing (e.g., checking for APKs, etc.)

	return nil
}

// Uncomment and implement the downloadTarball function if you need to download the source code tarball
// func downloadTarball(tarballURL, fileName string) error {
//     // Create the file
//     out, err := os.Create(fileName)
//     if err != nil {
//         return err
//     }
//     defer out.Close()

//     // Get the data
//     resp, err := http.Get(tarballURL)
//     if err != nil {
//         return err
//     }
//     defer resp.Body.Close()

//     // Write the body to file
//     _, err = io.Copy(out, resp.Body)
//     return err
// }

func main() {
	var (
		appsFilePath = flag.String("ap", "apps.yaml", "Path to apps.yaml file")
		repoDir      = flag.String("rd", "fdroid/repo", "Path to fdroid \"repo\" directory")
		accessToken  = flag.String("pat", "", "GitHub personal access token")

		debugMode = flag.Bool("debug", false, "Debug mode won't run the fdroid command")
	)
	flag.Parse()

	fmt.Println("::group::Initializing")

	appsList, err := apps.ParseAppFile(*appsFilePath)
	if err != nil {
		switch repo.Host {
		case "github.com":
			err = handleGitHubRepo(githubClient, repo)
		case "codeberg.org":
			err = handleCodebergRepo(repo)
		case "gitlab.com":
			err = handleGitLabRepo(repo)
		default:
			log.Printf("Unsupported host: %s", repo.Host)
			haveError = true
			return
	}
	
	if err != nil {
		log.Printf("Error handling repository %s/%s: %s", repo.Author, repo.Name, err.Error())
		haveError = true
		return
	}
	
	}

	var authenticatedClient *http.Client = nil
	if *accessToken != "" {
		ctx := context.Background()
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: *accessToken},
		)
		authenticatedClient = oauth2.NewClient(ctx, ts)
	}
	githubClient := github.NewClient(authenticatedClient)

	var haveError bool

	fdroidIndexFilePath := filepath.Join(*repoDir, "index-v1.json")

	initialFdroidIndex, err := apps.ReadIndex(fdroidIndexFilePath)
	if err != nil {
		log.Fatalf("reading f-droid repo index: %s\n", err.Error())
	}

	err = os.MkdirAll(*repoDir, 0o644)
	if err != nil {
		log.Fatalf("creating repo directory: %s\n", err.Error())
	}

	fmt.Println("::endgroup::")

	// map[apkName]info
	var apkInfoMap = make(map[string]apps.AppInfo)

	for _, app := range appsList {
		fmt.Printf("App: %s/%s\n", app.Author(), app.Name())

		repo, err := apps.RepoInfo(app.GitURL)
		if err != nil {
			log.Printf("Error while getting repo info from URL %q: %s", app.GitURL, err.Error())
			haveError = true
			return
		}

		log.Printf("Looking up %s/%s on GitHub", repo.Author, repo.Name)
		gitHubRepo, _, err := githubClient.Repositories.Get(context.Background(), repo.Author, repo.Name)
		if err != nil {
			log.Printf("Error while looking up repo: %s", err.Error())
		} else {
			app.Summary = gitHubRepo.GetDescription()

			if gitHubRepo.License != nil && gitHubRepo.License.SPDXID != nil {
				app.License = *gitHubRepo.License.SPDXID
			}

			log.Printf("Data from GitHub: summary=%q, license=%q", app.Summary, app.License)
		}

		releases, err := apps.ListAllReleases(githubClient, repo.Author, repo.Name)
		if err != nil {
			log.Printf("Error while listing repo releases for %q: %s\n", app.GitURL, err.Error())
			haveError = true
			return
		}

		log.Printf("Received %d releases", len(releases))

		for _, release := range releases {
			fmt.Printf("::group::Release %s\n", release.GetTagName())
			func() {
				defer fmt.Println("::endgroup::")

				if release.GetPrerelease() {
					log.Printf("Skipping prerelease %q", release.GetTagName())
					return
				}
				if release.GetDraft() {
					log.Printf("Skipping draft %q", release.GetTagName())
					return
				}
				if release.GetTagName() == "" {
					log.Printf("Skipping release with empty tag name")
					return
				}

				log.Printf("Working on release with tag name %q", release.GetTagName())

				apk := apps.FindAPKRelease(release)
				if apk == nil {
					log.Printf("Couldn't find a release asset with extension \".apk\"")
					return
				}

				appName := apps.GenerateReleaseFilename(app.Name(), release.GetTagName())

				log.Printf("Target APK name: %s", appName)

				appClone := app

				appClone.ReleaseDescription = release.GetBody()
				if appClone.ReleaseDescription != "" {
					log.Printf("Release notes: %s", appClone.ReleaseDescription)
				}

				apkInfoMap[appName] = appClone

				appTargetPath := filepath.Join(*repoDir, appName)

				// If the app file already exists for this version, we continue
				if _, err := os.Stat(appTargetPath); !errors.Is(err, os.ErrNotExist) {
					log.Printf("Already have APK for version %q at %q", release.GetTagName(), appTargetPath)
					return
				}

				log.Printf("Downloading APK %q from release %q to %q", apk.GetName(), release.GetTagName(), appTargetPath)

				dlCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				defer cancel()

				appStream, _, err := githubClient.Repositories.DownloadReleaseAsset(dlCtx, repo.Author, repo.Name, apk.GetID(), http.DefaultClient)
				if err != nil {
					log.Printf("Error while downloading app %q (artifact id %d) from from release %q: %s", app.GitURL, apk.GetID(), release.GetTagName(), err.Error())
					haveError = true
					return
				}

				err = downloadStream(appTargetPath, appStream)
				if err != nil {
					log.Printf("Error while downloading app %q (artifact id %d) from from release %q to %q: %s", app.GitURL, *apk.ID, *release.TagName, appTargetPath, err.Error())
					haveError = true
					return
				}

				log.Printf("Successfully downloaded app for version %q", release.GetTagName())
			}()
		}
	}

	if !*debugMode {
		fmt.Println("::group::F-Droid: Creating metadata stubs")
		// Now, we run the fdroid update command
		cmd := exec.Command("fdroid", "update", "--pretty", "--create-metadata", "--delete-unknown")
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		cmd.Stdin = os.Stdin
		cmd.Dir = filepath.Dir(*repoDir)

		log.Printf("Running %q in %s", cmd.String(), cmd.Dir)

		err = cmd.Run()

		if err != nil {
			log.Println("Error while running \"fdroid update -c\":", err.Error())

			fmt.Println("::endgroup::")
			os.Exit(1)
		}
		fmt.Println("::endgroup::")
	}

	fmt.Println("Filling in metadata")

	fdroidIndex, err := apps.ReadIndex(fdroidIndexFilePath)
	if err != nil {
		log.Fatalf("reading f-droid repo index: %s\n::endgroup::\n", err.Error())
	}

	// directory paths that should be removed after updating metadata
	var toRemovePaths []string

	walkPath := filepath.Join(filepath.Dir(*repoDir), "metadata")
	err = filepath.WalkDir(walkPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".yml") {
			return err
		}

		pkgname := strings.TrimSuffix(filepath.Base(path), ".yml")

		fmt.Printf("::group::%s\n", pkgname)

		return func() error {
			defer fmt.Println("::endgroup::")
			log.Printf("Working on %q", pkgname)

			meta, err := apps.ReadMetaFile(path)
			if err != nil {
				log.Printf("Reading meta file %q: %s", path, err.Error())
				return nil
			}

			latestPackage, ok := fdroidIndex.FindLatestPackage(pkgname)
			if !ok {
				return nil
			}

			log.Printf("The latest version is %q with versionCode %d", latestPackage.VersionName, latestPackage.VersionCode)

			apkInfo, ok := apkInfoMap[latestPackage.ApkName]
			if !ok {
				log.Printf("Cannot find apk info for %q", latestPackage.ApkName)
				return nil
			}

			// Now update with some info

			setNonEmpty(meta, "AuthorName", apkInfo.Author())
			fn := apkInfo.FriendlyName
			if fn == "" {
				fn = apkInfo.Name()
			}
			setNonEmpty(meta, "Name", fn)
			setNonEmpty(meta, "SourceCode", apkInfo.GitURL)
			setNonEmpty(meta, "License", apkInfo.License)
			setNonEmpty(meta, "Description", apkInfo.Description)

			var summary = apkInfo.Summary
			// See https://f-droid.org/en/docs/Build_Metadata_Reference/#Summary for max length
			const maxSummaryLength = 80
			if len(summary) > maxSummaryLength {
				summary = summary[:maxSummaryLength-3] + "..."

				log.Printf("Truncated summary to length of %d (max length)", len(summary))
			}

			setNonEmpty(meta, "Summary", summary)

			if len(apkInfo.Categories) != 0 {
				meta["Categories"] = apkInfo.Categories
			}

			if len(apkInfo.AntiFeatures) != 0 {
				meta["AntiFeatures"] = strings.Join(apkInfo.AntiFeatures, ",")
			}

			meta["CurrentVersion"] = latestPackage.VersionName
			meta["CurrentVersionCode"] = latestPackage.VersionCode

			log.Printf("Set current version info to versionName=%q, versionCode=%d", latestPackage.VersionName, latestPackage.VersionCode)

			err = apps.WriteMetaFile(path, meta)
			if err != nil {
				log.Printf("Writing meta file %q: %s", path, err.Error())
				return nil
			}

			log.Printf("Updated metadata file %q", path)

			if apkInfo.ReleaseDescription != "" {
				destFilePath := filepath.Join(walkPath, latestPackage.PackageName, "en-US", "changelogs", fmt.Sprintf("%d.txt", latestPackage.VersionCode))

				err = os.MkdirAll(filepath.Dir(destFilePath), os.ModePerm)
				if err != nil {
					log.Printf("Creating directory for changelog file %q: %s", destFilePath, err.Error())
					return nil
				}

				err = os.WriteFile(destFilePath, []byte(apkInfo.ReleaseDescription), os.ModePerm)
				if err != nil {
					log.Printf("Writing changelog file %q: %s", destFilePath, err.Error())
					return nil
				}

				log.Printf("Wrote release notes to %q", destFilePath)
			}

			log.Printf("Cloning git repository to search for screenshots")

			gitRepoPath, err := git.CloneRepo(apkInfo.GitURL)
			if err != nil {
				log.Printf("Cloning git repo from %q: %s", apkInfo.GitURL, err.Error())
				return nil
			}
			defer os.RemoveAll(gitRepoPath)

			metadata, err := apps.FindMetadata(gitRepoPath)
			if err != nil {
				log.Printf("finding metadata in git repo %q: %s", gitRepoPath, err.Error())
				return nil
			}

			log.Printf("Found %d screenshots", len(metadata.Screenshots))

			screenshotsPath := filepath.Join(walkPath, latestPackage.PackageName, "en-US", "phoneScreenshots")

			_ = os.RemoveAll(screenshotsPath)

			var sccounter int = 1
			for _, sc := range metadata.Screenshots {
				var ext = filepath.Ext(sc)
				if ext == "" {
					log.Printf("Invalid: screenshot file extension is empty for %q", sc)
					continue
				}

				var newFilePath = filepath.Join(screenshotsPath, fmt.Sprintf("%d%s", sccounter, ext))

				err = os.MkdirAll(filepath.Dir(newFilePath), os.ModePerm)
				if err != nil {
					log.Printf("Creating directory for screenshot file %q: %s", newFilePath, err.Error())
					return nil
				}

				err = file.Move(sc, newFilePath)
				if err != nil {
					log.Printf("Moving screenshot file %q to %q: %s", sc, newFilePath, err.Error())
					return nil
				}

				log.Printf("Wrote screenshot to %s", newFilePath)

				sccounter++
			}

			toRemovePaths = append(toRemovePaths, screenshotsPath)

			return nil
		}()
	})
	if err != nil {
		log.Printf("Error while walking metadata: %s", err.Error())

		os.Exit(1)
	}

	if !*debugMode {
		fmt.Println("::group::F-Droid: Reading updated metadata")

		// Now, we run the fdroid update command again to regenerate the index with our new metadata
		cmd := exec.Command("fdroid", "update", "--pretty", "--delete-unknown")
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		cmd.Stdin = os.Stdin
		cmd.Dir = filepath.Dir(*repoDir)

		log.Printf("Running %q in %s", cmd.String(), cmd.Dir)

		err = cmd.Run()
		if err != nil {
			log.Println("Error while running \"fdroid update -c\":", err.Error())

			fmt.Println("::endgroup::")
			os.Exit(1)
		}
		fmt.Println("::endgroup::")
	}

	fmt.Println("::group::Assessing changes")

	// Now at the end, we read the index again
	fdroidIndex, err = apps.ReadIndex(fdroidIndexFilePath)
	if err != nil {
		log.Fatalf("reading f-droid repo index: %s\n::endgroup::\n", err.Error())
	}

	// Now we can remove all paths that were marked for doing so

	for _, rmpath := range toRemovePaths {
		err = os.RemoveAll(rmpath)
		if err != nil {
			log.Fatalf("removing path %q: %s\n", rmpath, err.Error())
		}
	}

	// We can now generate the README file
	readmePath := filepath.Join(filepath.Dir(filepath.Dir(*repoDir)), "README.md")
	err = md.RegenerateReadme(readmePath, fdroidIndex)
	if err != nil {
		log.Fatalf("error generating %q: %s\n", readmePath, err.Error())
	}

	cpath, haveSignificantChanges := apps.HasSignificantChanges(initialFdroidIndex, fdroidIndex)
	if haveSignificantChanges {
		log.Printf("The index %q had a significant change at JSON path %q", fdroidIndexFilePath, cpath)
	} else {
		log.Printf("The index files didn't change significantly")

		changedFiles, err := git.GetChangedFileNames(*repoDir)
		if err != nil {
			log.Fatalf("getting changed files: %s\n::endgroup::\n", err.Error())
		}

		// If only the index files changed, we ignore the commit
		for _, fname := range changedFiles {
			if !strings.Contains(fname, "index") {
				haveSignificantChanges = true

				log.Printf("File %q is a significant change", fname)
			}
		}

		if !haveSignificantChanges {
			log.Printf("It doesn't look like there were any relevant changes, neither to the index file nor any file indexed by git.")
		}
	}

	fmt.Println("::endgroup::")

	// If we have an error, we report it as such
	if haveError {
		os.Exit(1)
	}

	// If we don't have any good changes, we report it with exit code 2
	if !haveSignificantChanges {
		os.Exit(2)
	}

	// If we have relevant changes, we exit with code 0
}

func setNonEmpty(m map[string]interface{}, key string, value string) {
	if value != "" || m[key] == "Unknown" {
		m[key] = value

		log.Printf("Set %s to %q", key, value)
	}
}

func downloadStream(targetFile string, rc io.ReadCloser) (err error) {
	defer rc.Close()

	targetTemp := targetFile + ".tmp"

	f, err := os.Create(targetTemp)
	if err != nil {
		return
	}

	_, err = io.Copy(f, rc)
	if err != nil {
		_ = f.Close()
		_ = os.Remove(targetTemp)

		return
	}

	err = f.Close()
	if err != nil {
		return
	}

	return os.Rename(targetTemp, targetFile)
}
