package logview

import (
	"archive/zip"
	"bytes"
	"cmp"
	"fmt"
	"io"
	"path"
	"regexp"
	"slices"
	"strings"
)

// GitHub Actions gives a run's logs as a zip (decision 99, T4): a
// folder per job holding a file per step ("build/3_Run npm test.txt"),
// and beside them each job's whole log ("0_build.txt"), which repeats
// the steps. The steps' files are read, each one's entries in the step
// its name gives; a job's whole log only when the job has no folder.

// stepFile reads GitHub's "3_Run npm test.txt".
var stepFile = regexp.MustCompile(`^(\d+)_(.+)\.txt$`)

// maxZipFiles bounds the files taken from one zip.
const maxZipFiles = 500

// zipped is a file taken out of a zip, with the step it is and the job
// it belongs to.
type zipped struct {
	File
	step, job string
}

// unzip expands a zip into its text files, in the zip's order with the
// steps of each job in their number's order. limit caps the bytes taken
// out (0: none), so a small zip can't fill memory; it is what is left of
// the limit of all the files opened together.
func unzip(f File, limit int64) ([]zipped, error) {
	zr, err := zip.NewReader(bytes.NewReader(f.Data), int64(len(f.Data)))
	if err != nil {
		return nil, fmt.Errorf("%s: reading zip: %w", f.Name, err)
	}
	jobs := map[string]bool{} // folders: jobs with step files
	for _, zf := range zr.File {
		if dir := path.Dir(zf.Name); dir != "." && stepFile.MatchString(path.Base(zf.Name)) {
			jobs[dir] = true
		}
	}
	type item struct {
		zf   *zip.File
		job  string
		n    int
		name string
	}
	var items []item
	for _, zf := range zr.File {
		if zf.FileInfo().IsDir() {
			continue
		}
		dir, base := path.Dir(zf.Name), path.Base(zf.Name)
		m := stepFile.FindStringSubmatch(base)
		switch {
		case dir == "." && m != nil && jobs[m[2]]:
			continue // a job's whole log; its steps are in its folder
		case m != nil && dir != ".":
			n := 0
			fmt.Sscan(m[1], &n)
			items = append(items, item{zf, dir, n, m[2]})
		case strings.HasSuffix(base, ".txt") || strings.HasSuffix(base, ".log"):
			items = append(items, item{zf: zf, job: dir})
		}
	}
	// Steps in their number's order within each job, jobs as the zip has them.
	jobAt := map[string]int{}
	for _, it := range items {
		if _, ok := jobAt[it.job]; !ok {
			jobAt[it.job] = len(jobAt)
		}
	}
	slices.SortStableFunc(items, func(a, b item) int { return cmp.Or(jobAt[a.job]-jobAt[b.job], a.n-b.n) })
	if len(items) > maxZipFiles {
		return nil, fmt.Errorf("%s: more than %d files in the zip", f.Name, maxZipFiles)
	}

	var out []zipped
	var total int64
	for _, it := range items {
		rc, err := it.zf.Open()
		if err != nil {
			return nil, fmt.Errorf("%s: %s: %w", f.Name, it.zf.Name, err)
		}
		r := io.Reader(rc)
		if limit > 0 {
			r = io.LimitReader(rc, limit-total+1)
		}
		data, err := io.ReadAll(r)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %s: %w", f.Name, it.zf.Name, err)
		}
		if total += int64(len(data)); limit > 0 && total > limit {
			return nil, ErrTooLarge
		}
		step := it.name
		if step != "" && len(jobs) > 1 {
			step = it.job + " · " + step
		}
		job := it.job
		if job == "." {
			job = ""
		}
		out = append(out, zipped{File{Name: f.Name + "/" + it.zf.Name, Data: data}, step, job})
	}
	return out, nil
}

// isZip reports a zip's signature.
func isZip(data []byte) bool { return bytes.HasPrefix(data, []byte("PK\x03\x04")) }
