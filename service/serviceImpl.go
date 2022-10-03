package service

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"mailPro/pojo"
	"time"

	"github.com/unidoc/unipdf/v3/common/license"
	"github.com/unidoc/unipdf/v3/creator"
	"github.com/unidoc/unipdf/v3/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	gomail "gopkg.in/mail.v2"
)

type Connection struct {
	Server     string
	Database   string
	Collection string
}

var Collection *mongo.Collection
var ctx = context.TODO()
var insertDocs int

func (e *Connection) Connect() {
	clientOptions := options.Client().ApplyURI(e.Server)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatal(err)
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	err = license.SetMeteredKey("72c4ab06d023bbc8b2e186d089f9e052654afea32b75141f39c7dc1ab3b108ca")
	if err != nil {
		log.Fatal(err)
	}

	Collection = client.Database(e.Database).Collection(e.Collection)
}

func (e *Connection) InsertAllData(emailData pojo.EmailData) (string, error) {

	/*	err := sendMail(emailData)
		if err != nil {
			log.Fatal(err)
		}*/

	emailData.Date = time.Now()
	_, err1 := Collection.InsertOne(ctx, emailData)

	if err1 != nil {
		log.Fatal(err1)
	}
	return "Email sent", err1
}

func sendMail(emailData pojo.EmailData) error {
	m := gomail.NewMessage()
	m.SetHeaders(map[string][]string{
		"From":    {m.FormatAddress("anurag.singh@gridinfocom.com", "Anurag")},
		"To":      emailData.EmailTo,
		"Cc":      emailData.EmailCC,
		"Subject": emailData.EmailSubject,
	})

	m.SetBody("text/plain", emailData.EmailBody)
	files, err := ioutil.ReadDir("./uploads/")
	if err != nil {
		log.Fatal(err)
	}
	//	filePath := "./uploads"
	for _, file := range files {

		m.Attach("./uploads/" + file.Name())
	}

	// Settings for SMTP server
	d := gomail.NewDialer("smtp-relay.sendinblue.com", 587, "anurag.singh@gridinfocom.com", "UdKaQS7b4c53OC1W")

	d.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	// Now send E-Mail
	if err := d.DialAndSend(m); err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}

func (e *Connection) SearchAllMails(bodyData pojo.SerarchData) ([]*pojo.EmailData, error) {
	var searchData []*pojo.EmailData

	filter := bson.D{}

	if bodyData.EmailTo != "" {
		filter = append(filter, primitive.E{Key: "email_to", Value: bson.M{"$regex": bodyData.EmailTo}})
	}
	if bodyData.EmailCC != "" {
		filter = append(filter, primitive.E{Key: "email_cc", Value: bson.M{"$regex": bodyData.EmailCC}})
	}
	if bodyData.EmailBCC != "" {
		filter = append(filter, primitive.E{Key: "email_bcc", Value: bson.M{"$regex": bodyData.EmailBCC}})
	}
	if bodyData.EmailSubject != "" {
		filter = append(filter, primitive.E{Key: "email_subject", Value: bson.M{"$regex": bodyData.EmailSubject}})
	}

	t, _ := time.Parse("2006-01-02", bodyData.Date)
	if bodyData.Date != "" {
		filter = append(filter, primitive.E{Key: "date", Value: bson.M{
			"$gte": primitive.NewDateTimeFromTime(t)}})
	}

	result, err := Collection.Find(ctx, filter)

	if err != nil {
		return searchData, err
	}

	for result.Next(ctx) {
		var data pojo.EmailData
		err := result.Decode(&data)
		if err != nil {
			return searchData, err
		}
		searchData = append(searchData, &data)
	}

	if searchData == nil {
		return searchData, errors.New("Data Not Found In DB")
	}
	dir := "download/"
	file := "test"
	_, errPdf := writeDataIntoPDFTable(dir, file, searchData)
	if errPdf != nil {
		fmt.Println(errPdf)
	}

	err1 := sendMail1("test.pdf")
	if err != nil {
		log.Fatal(err1)
	}
	return searchData, nil
}

func writeDataIntoPDFTable(dir, file string, data []*pojo.EmailData) (*creator.Creator, error) {

	c := creator.New()
	c.SetPageMargins(10, 10, 10, 10)

	// Create report fonts.
	// UniPDF supports a number of font-families, which can be accessed using model.
	// Here we are creating two fonts, a normal one and its bold version
	font, err := model.NewStandard14Font(model.HelveticaName)
	if err != nil {
		return c, err
	}

	// Bold font
	fontBold, err := model.NewStandard14Font(model.HelveticaBoldName)
	if err != nil {
		return c, err
	}

	// Generate basic usage chapter.
	if err := basicUsage(c, font, fontBold, data); err != nil {
		return c, err
	}

	err = c.WriteToFile(dir + file + ".pdf")
	if err != nil {
		return c, err
	}
	return c, nil
}

func basicUsage(c *creator.Creator, font, fontBold *model.PdfFont, data []*pojo.EmailData) error {
	// Create chapter.
	ch := c.NewChapter("Search Data")
	ch.SetMargins(0, 0, 10, 0)
	ch.GetHeading().SetFont(font)
	ch.GetHeading().SetFontSize(18)
	ch.GetHeading().SetColor(creator.ColorRGBFrom8bit(72, 86, 95))
	// You can also set inbuilt colors using creator
	// ch.GetHeading().SetColor(creator.ColorBlack)

	// Draw subchapters. Here we are only create horizontally aligned chapter.
	// You can also vertically align and perform other optimizations as well.
	// Check GitHub example for more.
	contentAlignH(c, ch, font, fontBold, data)

	// Draw chapter.
	if err := c.Draw(ch); err != nil {
		return err
	}

	return nil
}

func contentAlignH(c *creator.Creator, ch *creator.Chapter, font, fontBold *model.PdfFont, data []*pojo.EmailData) {
	// Create subchapter.
	// sc := ch.NewSubchapter("Content horizontal alignment")
	// sc.GetHeading().SetFontSize(10)
	// sc.GetHeading().SetColor(creator.ColorBlue)

	// Create table.
	table := c.NewTable(7)
	table.SetMargins(0, 0, 15, 0)

	drawCell := func(text string, font *model.PdfFont, align creator.CellHorizontalAlignment) {
		p := c.NewStyledParagraph()
		p.Append(text).Style.Font = font

		cell := table.NewCell()
		cell.SetBorder(creator.CellBorderSideAll, creator.CellBorderStyleSingle, 1)
		cell.SetHorizontalAlignment(align)
		cell.SetContent(p)
	}
	// Draw table header.
	drawCell("ID", fontBold, creator.CellHorizontalAlignmentLeft)
	drawCell("EmailTo", fontBold, creator.CellHorizontalAlignmentCenter)
	drawCell("EmailCC", fontBold, creator.CellHorizontalAlignmentRight)
	drawCell("EmailBCC", fontBold, creator.CellHorizontalAlignmentLeft)
	drawCell("EmailSubject", fontBold, creator.CellHorizontalAlignmentRight)
	drawCell("EmailBody", fontBold, creator.CellHorizontalAlignmentLeft)
	drawCell("Date", fontBold, creator.CellHorizontalAlignmentCenter)

	// Draw table content.
	for i := range data {

		/*drawCell(fmt.Sprintf("%v", data[i].ID), font, creator.CellHorizontalAlignmentLeft)
		for j := range data[i].EmailTo {
			drawCell(data[i].EmailTo[j], font, creator.CellHorizontalAlignmentCenter)
		}
		for j := range data[i].EmailCC {
			drawCell(data[i].EmailCC[j], font, creator.CellHorizontalAlignmentCenter)
		}
		for j := range data[i].EmailBCC {
			drawCell(data[i].EmailBCC[j], font, creator.CellHorizontalAlignmentCenter)
		}
		for j := range data[i].EmailSubject {
			drawCell(data[i].EmailSubject[j], font, creator.CellHorizontalAlignmentCenter)
		}
		drawCell(data[i].EmailBody, font, creator.CellHorizontalAlignmentCenter)
		//	drawCell(data[i].Date, font, creator.CellHorizontalAlignmentCenter)*/
		emailTostr := ""
		emailCcstr := ""
		emailBccStr := ""
		emailSubjectstr := ""
		y := 0
		to := data[i].EmailTo
		for _, v := range to {
			if y != 0 {
				emailTostr = emailTostr + ","
			}
			emailTostr = emailTostr + v
			y++
		}
		y = 0
		for _, v1 := range data[i].EmailCC {
			if y != 0 {
				emailTostr = emailTostr + ","
			}
			emailCcstr = emailCcstr + v1
			y++
		}
		y = 0
		for _, v2 := range data[i].EmailBCC {
			if y != 0 {
				emailTostr = emailTostr + ","
			}
			emailBccStr = emailBccStr + v2
			y++
		}
		y = 0
		for _, v3 := range data[i].EmailSubject {
			if y != 0 {
				emailTostr = emailTostr + ","
			}
			emailSubjectstr = emailSubjectstr + v3
			y++
		}
		drawCell(fmt.Sprintf("%v", data[i].ID), font, creator.CellHorizontalAlignmentCenter)
		drawCell(emailTostr, font, creator.CellHorizontalAlignmentCenter)
		drawCell(emailCcstr, font, creator.CellHorizontalAlignmentCenter)
		drawCell(emailBccStr, font, creator.CellHorizontalAlignmentCenter)
		drawCell(emailSubjectstr, font, creator.CellHorizontalAlignmentCenter)
		drawCell(data[i].EmailBody, font, creator.CellHorizontalAlignmentCenter)
		//	drawCell(fmt.Sprintf("%v", data[i].PinCode), font, creator.CellHorizontalAlignmentCenter)
		drawCell(data[i].Date.String(), font, creator.CellHorizontalAlignmentCenter)
		//	drawCell(fmt.Sprintf("%v", data[i].CategoriesId), font, creator.CellHorizontalAlignmentCenter)
	}

	ch.Add(table)
}

func (e *Connection) MailById(id string) ([]*pojo.EmailData, error) {
	var searchData []*pojo.EmailData
	id2, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return searchData, err
	}
	filter := bson.D{}

	filter = append(filter, primitive.E{Key: "_id", Value: id2})

	result, err := Collection.Find(ctx, filter)
	fmt.Println(result)
	if err != nil {
		return searchData, err
	}

	for result.Next(ctx) {
		var data pojo.EmailData
		err := result.Decode(&data)
		if err != nil {
			return searchData, err
		}
		searchData = append(searchData, &data)
	}
	fmt.Println(searchData)
	if searchData == nil {
		return searchData, errors.New("Data Not Found In DB")
	}
	dir := "download/"
	file := "test"
	_, errPdf := writeDataIntoPDFTable(dir, file, searchData)
	if errPdf != nil {
		fmt.Println(errPdf)
	}

	return searchData, nil
}

func sendMail1(file string) error {
	m := gomail.NewMessage()
	m.SetHeaders(map[string][]string{
		"From":    {m.FormatAddress("anurag.singh@gridinfocom.com", "Anurag")},
		"To":      {"ramashankar.kumar@gridinfocom.com", "anurag.singh@gridinfocom.com"},
		"Cc":      {},
		"Subject": {"This is for testing"},
	})

	m.SetBody("text/plain", `Hii Ramashankar,
	
	                             This mail body is for testing
								 
								 regards,
								 Anurag`)
	m.Attach(file)
	m.Attach("Goal sheet.numbers")
	// Settings for SMTP server
	d := gomail.NewDialer("smtp-relay.sendinblue.com", 587, "anurag.singh@gridinfocom.com", "UdKaQS7b4c53OC1W")

	d.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	// Now send E-Mail
	if err := d.DialAndSend(m); err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}
