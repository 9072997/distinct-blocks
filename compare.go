package main

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/mitchellh/ioprogress"
)

func hash(b []byte) [16]byte {
	s := md5.New()
	s.Write(b)
	h := s.Sum(nil)
	var r [16]byte
	copy(r[:], h)
	return r
}

func distinct(readers []io.Reader, blockSize int) (distinct, total, zero int, err error) {
	blocks := make(map[[16]byte]struct{})
	buf := make([]byte, blockSize)
	zeroHash := hash(make([]byte, blockSize))

	for _, r := range readers {
		for {
			n, err := io.ReadFull(r, buf)
			if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
				return 0, 0, 0, err
			}
			if n == 0 {
				break
			}
			total++
			hash := hash(buf[:n])
			blocks[hash] = struct{}{}
			if hash == zeroHash {
				zero++
			}
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
		}
	}

	return len(blocks), total, zero, nil
}

func drawFunc(label string) ioprogress.DrawFunc {
	var timeMeasurements [10]time.Time
	var progressMeasurements [10]int64
	var idx int
	return ioprogress.DrawTerminalf(
		os.Stderr,
		func(progress, total int64) string {
			now := time.Now()
			var remainingStr string
			if progressMeasurements[idx] == 0 {
				remainingStr = "..."
			} else {
				tenTime := now.Sub(timeMeasurements[idx])
				tenProgress := progress - progressMeasurements[idx]
				tenSpeed := float64(tenProgress) / float64(tenTime)
				progressLeft := float64(total - progress)
				remaining := time.Duration(progressLeft / tenSpeed)
				// truncate to seconds
				remaining = remaining / time.Second * time.Second
				remainingStr = remaining.String()
			}
			timeMeasurements[idx] = now
			progressMeasurements[idx] = progress
			idx = (idx + 1) % len(timeMeasurements)

			bar := ioprogress.DrawTextFormatBar(20)
			return fmt.Sprintf(
				"%s: %s %s (%s)",
				label,
				bar(progress, total),
				ioprogress.DrawTextFormatBytes(progress, total),
				remainingStr,
			)
		},
	)
}

// parse "4m", "2k", "1024", etc
func parseSize(s string) (int, error) {
	suffix := s[len(s)-1]
	switch suffix {
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		b, err := strconv.Atoi(s)
		if err != nil {
			return 0, err
		}
		return b, nil
	case 'k', 'K':
		kb, err := strconv.Atoi(s[:len(s)-1])
		if err != nil {
			return 0, err
		}
		return kb * 1024, nil
	case 'm', 'M':
		mb, err := strconv.Atoi(s[:len(s)-1])
		if err != nil {
			return 0, err
		}
		return mb * 1024 * 1024, nil
	case 'g', 'G':
		gb, err := strconv.Atoi(s[:len(s)-1])
		if err != nil {
			return 0, err
		}
		return gb * 1024 * 1024 * 1024, nil
	case 't', 'T':
		tb, err := strconv.Atoi(s[:len(s)-1])
		if err != nil {
			return 0, err
		}
		return tb * 1024 * 1024 * 1024 * 1024, nil
	default:
		return 0, fmt.Errorf("invalid size suffix: %q", suffix)
	}
}

func humanizeSize(b int) string {
	if b < 1024 {
		return fmt.Sprintf("%d", b)
	}
	if b < 1024*1024 {
		return fmt.Sprintf("%.1fK", float64(b)/1024)
	}
	if b < 1024*1024*1024 {
		return fmt.Sprintf("%.1fM", float64(b)/(1024*1024))
	}
	if b < 1024*1024*1024*1024 {
		return fmt.Sprintf("%.1fG", float64(b)/(1024*1024*1024))
	}
	return fmt.Sprintf("%.1fT", float64(b)/(1024*1024*1024*1024))
}

func showHelp() {
	fmt.Printf("usage: %s <block size> <file>...\n", os.Args[0])
	fmt.Printf("example: %s 4m file1 file2\n", os.Args[0])
	os.Exit(1)
}

func main() {
	if len(os.Args) < 3 {
		showHelp()
	}

	blockSize, err := parseSize(os.Args[1])
	if err != nil {
		showHelp()
	}

	var files []io.Reader
	for _, arg := range os.Args[2:] {
		file, err := os.Open(arg)
		if err != nil {
			panic(err)
		}
		defer file.Close()

		// progress bars
		info, err := file.Stat()
		if err != nil {
			panic(err)
		}
		progressReader := &ioprogress.Reader{
			Reader:   file,
			Size:     info.Size(),
			DrawFunc: drawFunc(info.Name()),
		}
		files = append(files, progressReader)
	}

	distinct, total, zero, err := distinct(files, blockSize)
	if err != nil {
		panic(err)
	}

	totalData := total * blockSize
	fmt.Printf(
		"total blocks: %d, %s\n",
		total,
		humanizeSize(totalData),
	)
	nonZero := total - zero
	nonZeroData := nonZero * blockSize
	fmt.Printf(
		"non-zero blocks: %d, %s\n",
		zero,
		humanizeSize(nonZeroData),
	)
	fmt.Printf(
		"\t%.2f%% of total\n",
		float64(nonZero)/float64(total)*100,
	)
	distinctData := distinct * blockSize
	fmt.Printf(
		"distinct blocks: %d, %s\n",
		distinct,
		humanizeSize(distinctData),
	)
	fmt.Printf(
		"\t%.2f%% of total\n",
		float64(distinct)/float64(total)*100,
	)
	fmt.Printf(
		"\t%.2f%% of non-zero\n",
		float64(distinct)/float64(nonZero)*100,
	)
}
