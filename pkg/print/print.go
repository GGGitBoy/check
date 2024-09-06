package print

import (
	"bytes"
	"fmt"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/signintech/gopdf"
	"github.com/sirupsen/logrus"
	"image"
	"image/draw"
	"image/png"
	"inspection-server/pkg/common"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

var (
	waitSecond  = 30
	waitTimeOut = 60
)

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

	if common.PrintWaitTimeOut != "" {
		num, err := strconv.Atoi(common.PrintWaitTimeOut)
		if err != nil {
			logrus.Errorf("Invalid PrintWaitTimeOut value, using default: %v", err)
		} else {
			waitTimeOut = num
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
	err = page.Timeout(time.Duration(waitTimeOut) * time.Minute).WaitLoad()
	if err != nil {
		return fmt.Errorf("Failed to wait load: %v\n", err)
	}
	time.Sleep(time.Duration(waitSecond) * time.Second)

	logrus.Infof("[%s] Starting page scroll", taskName)

	// 获取页面总高度
	metrics, err := page.Timeout(time.Duration(waitTimeOut) * time.Minute).Eval(`() => ({ 
			width: document.body.scrollWidth, 
			height: document.body.scrollHeight 
	})`)
	if err != nil {
		return fmt.Errorf("Failed to get page dimensions: %v\n", err)
	}

	viewPageWidth := metrics.Value.Get("width").Int()
	viewPageHeight := metrics.Value.Get("height").Int()
	logrus.Infof("[%s] Page dimensions: width=%d, height=%d", taskName, viewPageWidth, viewPageHeight)

	var allScreenshots []image.Image
	viewportHeight := 5000
	currentHeight := 0
	for currentHeight < viewPageHeight {
		// 设置当前视口的大小
		page.MustSetViewport(viewPageWidth, viewportHeight, 1, false)

		// 滚动到当前高度
		_, err = page.Timeout(time.Duration(waitTimeOut) * time.Minute).Eval(fmt.Sprintf(`() => {window.scrollTo(0, %d);}`, currentHeight))
		if err != nil {
			return fmt.Errorf("Failed to scroll window: %v\n", err)
		}

		time.Sleep(time.Duration(waitSecond) * time.Second)

		// 截图当前视口
		screenshot, err := page.Screenshot(false, nil)
		if err != nil {
			return fmt.Errorf("Failed to capture screenshot: %v\n", err)
		}

		logrus.Infof("[%s] Screenshot captured successfully: width = %d, height = %d", taskName, viewPageWidth, currentHeight)

		// 将截图解码为图像对象
		img, err := png.Decode(bytes.NewReader(screenshot))
		if err != nil {
			return fmt.Errorf("Failed to decode screenshot: %v\n", err)
		}

		// 检查图像是否有效
		if img.Bounds().Dx() == 0 || img.Bounds().Dy() == 0 {
			logrus.Warnf("Skipping invalid screenshot")
			continue
		}

		// 将每张截图保存到数组中
		allScreenshots = append(allScreenshots, img)

		// 更新当前高度，继续下一个视口的截图
		currentHeight += viewportHeight
		time.Sleep(2 * time.Second) // 等待一段时间确保页面加载
	}

	// 拼接所有截图
	finalImage := stitchImagesVertically(allScreenshots)

	pngPath := common.PrintPDFPath + common.GetShotName(print.ReportTime)
	err = os.MkdirAll(filepath.Dir(pngPath), 0755)
	if err != nil {
		return fmt.Errorf("Failed to create directories for path: %s, error: %v\n", path, err)
	}

	outFile, err := os.Create(pngPath)
	if err != nil {
		return fmt.Errorf("Failed to create file at path: %s, error: %v\n", path, err)
	}
	defer func() {
		if cerr := outFile.Close(); cerr != nil {
			logrus.Errorf("Failed to close file at path: %s, error: %v", path, cerr)
		}
	}()

	err = png.Encode(outFile, finalImage)
	if err != nil {
		return fmt.Errorf("Failed to save final image: %v\n", err)
	}

	logrus.Infof("[%s] Starting create PDF", taskName)

	imgFile, err := os.Open(pngPath)
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

	err = pdf.Image(common.PrintPDFPath+common.GetShotName(print.ReportTime), 0, 0, rect)
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

// stitchImagesVertically 将多张图片垂直拼接为一张完整的图像
func stitchImagesVertically(images []image.Image) image.Image {
	totalHeight := 0
	width := 0

	// 计算拼接后的总高度和宽度
	for _, img := range images {
		totalHeight += img.Bounds().Dy()
		if img.Bounds().Dx() > width {
			width = img.Bounds().Dx()
		}
	}

	// 创建拼接后的图像
	finalImage := image.NewRGBA(image.Rect(0, 0, width, totalHeight))

	// 按顺序将每张图像绘制到最终图像上
	yOffset := 0
	for _, img := range images {
		draw.Draw(finalImage, image.Rect(0, yOffset, img.Bounds().Dx(), yOffset+img.Bounds().Dy()), img, image.Point{0, 0}, draw.Src)
		yOffset += img.Bounds().Dy()
	}

	return finalImage
}
