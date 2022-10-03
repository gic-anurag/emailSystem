package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mailPro/pojo"
	"mailPro/service"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	gomail "gopkg.in/mail.v2"
)

var mongoDetails = service.Connection{}

func init() {
	mongoDetails.Server = "mongodb://localhost:27017"
	mongoDetails.Database = "email_system"
	mongoDetails.Collection = "sent_mails"

	mongoDetails.Connect()
}

func main() {
	http.HandleFunc("/", sendMail)
	http.HandleFunc("/search-mails", searchMail)
	http.HandleFunc("/search-mailsById/", searchMailById)
	http.HandleFunc("/upload-attach-send/", searchMailById)
	http.HandleFunc("/upload-attach-send", attachDoc)
	http.HandleFunc("/upload-and-attach", uploadHandler)
	fmt.Println("Service Started at 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

const maxUploadSize = 10 * 1024 * 1024 // 10 mb
const uploadPath = "./demo"

/////

func respondWithJson(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

// //
func respondWithError(w http.ResponseWriter, code int, msg string) {
	respondWithJson(w, code, map[string]string{"error": msg})
}

///////////////

func sendMail(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	if r.Method != "POST" {
		respondWithError(w, http.StatusBadRequest, "Invalid method")
		return
	}

	var emailData pojo.EmailData

	if err := json.NewDecoder(r.Body).Decode(&emailData); err != nil {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("%v", err))
		return
	}

	if inserted, err := mongoDetails.InsertAllData(emailData); err != nil {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("%v", err))
	} else {
		respondWithJson(w, http.StatusAccepted, map[string]string{
			"message": inserted + "  Record Inserted Successfully",
		})
	}
}

func searchMail(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	if r.Method != "GET" {
		respondWithError(w, http.StatusBadRequest, "Invalid method")
		return
	}

	var serarchData pojo.SerarchData

	if err := json.NewDecoder(r.Body).Decode(&serarchData); err != nil {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("%v", err))
		return
	}

	if result, err := mongoDetails.SearchAllMails(serarchData); err != nil {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("%v", err))
	} else {
		respondWithJson(w, http.StatusAccepted, result)
	}
}

func searchMailById(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	if r.Method != "GET" {
		respondWithError(w, http.StatusBadRequest, "Invalid method")
		return
	}

	path := r.URL.Path

	arr := strings.Split(path, "/")
	id := arr[len(arr)-1]

	if result, err := mongoDetails.MailById(id); err != nil {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("%v", err))
	} else {
		respondWithJson(w, http.StatusAccepted, result)
	}
}

func attachDoc(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var emailData pojo.EmailData

	if err := json.NewDecoder(r.Body).Decode(&emailData); err != nil {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("%v", err))
		return
	}

	// 32 MB is the default used by FormFile()
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get a reference to the fileHeaders.
	// They are accessible only after ParseMultipartForm is called
	files1 := r.MultipartForm.File["file"]

	for _, fileHeader := range files1 {
		// Restrict the size of each uploaded file given size.
		// To prevent the aggregate size from exceeding
		// a specified value, use the http.MaxBytesReader() method
		// before calling ParseMultipartForm()
		if fileHeader.Size > maxUploadSize {
			http.Error(w, fmt.Sprintf("The uploaded image is too big: %s. Please use an image less than 1MB in size", fileHeader.Filename), http.StatusBadRequest)
			return
		}

		// Open the file
		file2, err := fileHeader.Open()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		defer file2.Close()

		buff := make([]byte, 512)
		_, err = file2.Read(buff)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		//filetype := http.DetectContentType(buff)
		// if filetype != "image/jpeg" && filetype != "image/png" {
		// 	http.Error(w, "The provided file format is not allowed. Please upload a JPEG or PNG image", http.StatusBadRequest)
		// 	return
		// }

		_, err = file2.Seek(0, io.SeekStart)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = os.MkdirAll("./uploads", os.ModePerm)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		f, err := os.Create(fmt.Sprintf("./uploads/%d%s", time.Now().UnixNano(), filepath.Ext(fileHeader.Filename)))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		defer f.Close()

		_, err = io.Copy(f, file2)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

	}

	fmt.Fprintf(w, "Upload successful")
	SendMail2(emailData)

}

func SendMail2(emailData pojo.EmailData) error {
	m := gomail.NewMessage()
	m.SetHeaders(map[string][]string{
		"From":    {m.FormatAddress("anurag.singh@gridinfocom.com", "Anurag")},
		"To":      emailData.EmailTo,
		"Cc":      emailData.EmailCC,
		"Bcc":     emailData.EmailBCC,
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
	for i := range emailData.FileLocation {
		m.Attach(emailData.FileLocation[i])
	}
	

	// Settings for SMTP server
	d := gomail.NewDialer("smtp-relay.sendinblue.com", 587, "anurag.singh@gridinfocom.com", "UdKaQS7b4c53OC1W")

	d.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	// Now send E-Mail
	if err := d.DialAndSend(m); err != nil {
		fmt.Println(err)
		return err
	}
	fmt.Println("email sent")
	return nil
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	files := r.MultipartForm.File["file"]
	body := r.MultipartForm.Value["request"][0]

	var emailData pojo.EmailData
	err := json.Unmarshal([]byte(body), &emailData)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("%v", err))
		return
	}
	// 32 MB is the default used by FormFile()
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get a reference to the fileHeaders.
	// They are accessible only after ParseMultipartForm is called

	for _, fileHeader := range files {
		// Restrict the size of each uploaded file given size.
		// To prevent the aggregate size from exceeding
		// a specified value, use the http.MaxBytesReader() method
		// before calling ParseMultipartForm()
		if fileHeader.Size > maxUploadSize {
			http.Error(w, fmt.Sprintf("The uploaded image is too big: %s. Please use an image less than 1MB in size", fileHeader.Filename), http.StatusBadRequest)
			return
		}

		// Open the file
		file, err := fileHeader.Open()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		defer file.Close()

		buff := make([]byte, 512)
		_, err = file.Read(buff)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		//filetype := http.DetectContentType(buff)
		// if filetype != "image/jpeg" && filetype != "image/png" {
		// 	http.Error(w, "The provided file format is not allowed. Please upload a JPEG or PNG image", http.StatusBadRequest)
		// 	return
		// }

		_, err = file.Seek(0, io.SeekStart)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = os.MkdirAll("./uploads", os.ModePerm)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		f, err := os.Create(fmt.Sprintf("./uploads/%d%s", time.Now().UnixNano(), filepath.Ext(fileHeader.Filename)))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		defer f.Close()

		_, err = io.Copy(f, file)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	fmt.Fprintf(w, "Upload successful")
	SendMail2(emailData)
}

/*func sendEmailAttach(w http.ResponseWriter, r *http.Request) {
	//	defer r.Body.Close()

	if r.Method != "POST" {
		respondWithError(w, http.StatusBadRequest, "Invalid method")
		return
	}
	fmt.Println("Request")
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "The uploaded file is too big. Please choose an file that's less than 1MB in size", http.StatusBadRequest)
		return

	}
	files := r.MultipartForm.File["file"]
	body := r.MultipartForm.Value["request"][0]

	var emailBody pojo.EmailData
	err := json.Unmarshal([]byte(body), &emailBody)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("%v", err))
		return
	}

	if len(emailBody.EmailTo) == 0 || emailBody.EmailBody == "" || len(emailBody.EmailSubject) == 0 {
		respondWithError(w, http.StatusBadGateway, "Please enter emailTo, email body and emailSubject")
		return
	}
	fmt.Println("Email:", emailBody)
	if result, err := mongoDetails.(emailBody, files); err != nil {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("%v", err))
	} else {
		respondWithJson(w, http.StatusAccepted, map[string]string{
			"message": result,
		})
	}
}*/
