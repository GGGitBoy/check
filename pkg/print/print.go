package print

import (
	"fmt"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/signintech/gopdf"
	"github.com/sirupsen/logrus"
	"image"
	"inspection-server/pkg/common"
	"os"
	"strconv"
	"time"
)

var waitSecond = 60

type Print struct {
	URL        string `json:"url"`
	ReportTime string `json:"report_time"`
}

func NewPrint() *Print {
	return &Print{}
}

func FullScreenshot(print *Print, taskName string) error {
	time.Sleep(2 * time.Second)
	if common.PrintWaitSecond != "" {
		num, err := strconv.Atoi(common.PrintWaitSecond)
		if err != nil {
			logrus.Errorf("Invalid PrintWaitSecond value, using default: %v", err)
		} else {
			waitSecond = num
		}
	}

	path, ok := launcher.LookPath()
	if !ok {
		return fmt.Errorf("Failed to find browser path\n")
	}
	u, err := launcher.New().Bin(path).NoSandbox(true).Launch()
	if err != nil {
		return fmt.Errorf("Failed to get launch: %v\n", err)
	}

	browser := rod.New().ControlURL(u)
	browser.Connect()
	if err != nil {
		return fmt.Errorf("Failed to connect: %v\n", err)
	}
	defer browser.Close()

	logrus.Infof("[%s] Starting page load", taskName)
	page, err := browser.Page(proto.TargetCreateTarget{URL: print.URL})
	if err != nil {
		return fmt.Errorf("Failed to get page: %v\n", err)
	}

	logrus.Infof("[%s] Starting wait load", taskName)
	err = page.Timeout(15 * time.Minute).WaitLoad()
	if err != nil {
		return fmt.Errorf("Failed to wait load: %v\n", err)
	}

	time.Sleep(time.Duration(waitSecond) * time.Second)
	logrus.Infof("[%s] Starting page scroll", taskName)

	_, err = page.Timeout(15 * time.Minute).Eval(`() => {
		var totalHeight = 0;
		var distance = 100;
		var timer = setInterval(() => {
			var scrollHeight = document.body.scrollHeight;
			window.scrollBy(0, distance);
			totalHeight += distance;
			if(totalHeight >= scrollHeight){
				clearInterval(timer);
			}
		}, 2000);
	}`)
	if err != nil {
		return fmt.Errorf("Failed page scroll: %v\n", err)
	}

	time.Sleep(time.Duration(waitSecond) * time.Second)

	logrus.Infof("[%s] Starting page wait scroll end", taskName)
	err = page.Timeout(15 * time.Minute).Wait(rod.Eval(`() => document.body.scrollHeight <= (window.scrollY + window.innerHeight)`))
	if err != nil {
		return fmt.Errorf("Error while waiting for page scroll completion: %v\n", err)
	}

	logrus.Infof("[%s] Starting get page width, height", taskName)

	metrics, err := page.Timeout(15 * time.Minute).Eval(`() => ({
		width: document.body.scrollWidth,
		height: document.body.scrollHeight,
	})`)
	if err != nil {
		return fmt.Errorf("Failed get page width, height: %v\n", err)
	}

	logrus.Infof("[%s] Page dimensions: width=%d, height=%d", taskName, metrics.Value.Get("width").Int(), metrics.Value.Get("height").Int())

	page.MustSetViewport(metrics.Value.Get("width").Int(), metrics.Value.Get("height").Int(), 1, false)

	screenshot, err := page.Screenshot(false, nil)
	if err != nil {
		return fmt.Errorf("Failed to capture screenshot: %v\n", err)
	}
	logrus.Infof("[%s] Screenshot captured successfully", taskName)

	err = common.WriteFile(common.PrintShotPath, screenshot)
	if err != nil {
		return fmt.Errorf("Failed to save screenshot: %v\n", err)
	}

	logrus.Infof("[%s] Starting create PDF", taskName)

	imgFile, err := os.Open(common.PrintShotPath)
	if err != nil {
		return fmt.Errorf("Failed to open screenshot file: %v\n", err)
	}
	defer imgFile.Close()

	img, _, err := image.Decode(imgFile)
	if err != nil {
		return fmt.Errorf("Failed to decode image: %v\n", err)
	}

	imgWidth := img.Bounds().Dx()
	imgHeight := img.Bounds().Dy()

	pageWidth := 595.28
	scale := pageWidth / float64(imgWidth)
	newHeight := float64(imgHeight) * scale

	pdf := gopdf.GoPdf{}
	rect := &gopdf.Rect{
		W: pageWidth,
		H: newHeight,
	}
	pdf.Start(gopdf.Config{PageSize: *rect})
	pdf.AddPage()

	err = pdf.Image(common.PrintShotPath, 0, 0, rect)
	if err != nil {
		return fmt.Errorf("Failed to add image to PDF: %v\n", err)
	}

	err = pdf.WritePdf(common.PrintPDFPath + common.GetReportFileName(print.ReportTime))
	if err != nil {
		return fmt.Errorf("Failed to save PDF: %v\n", err)
	}

	logrus.Infof("[%s] PDF generated successfully", taskName)
	return nil
}
