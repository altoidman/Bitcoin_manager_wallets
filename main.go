package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcwallet/walletdb"
	"github.com/btcsuite/btcwallet/wtxmgr"
	"github.com/gotk3/gotk3/gtk"
)

// ==================== هياكل البيانات ====================

type Wallet struct {
	mu       sync.Mutex
	Name     string
	FilePath string
	DB       walletdb.DB
}

type Transaction struct {
	ID        string
	From      string
	To        string
	Amount    float64
	Fee       float64
	Timestamp time.Time
	Status    string
}

type PriceService struct {
	CurrentPrice float64
	LastUpdated  time.Time
}

// ==================== المتغيرات العامة ====================

var (
	wallets        = make(map[string]*Wallet)
	currentWallet  *Wallet
	priceService   PriceService
	mainWindow     *gtk.Window
	walletListBox  *gtk.ListBox
	priceLabel     *gtk.Label
	statusLabel    *gtk.Label
	transactions   []Transaction
	networkParams = &chaincfg.MainNetParams
)

// ==================== وظائف المحفظة الأساسية ====================

func CreateWallet(name string) (*Wallet, error) {
	wallet := &Wallet{
		Name:     name,
		FilePath: filepath.Join("wallets", name+".db"),
	}

	// إنشاء مجلد المحافظ إذا لم يكن موجوداً
	if err := os.MkdirAll("wallets", 0700); err != nil {
		return nil, err
	}

	// إنشاء قاعدة بيانات المحفظة
	db, err := walletdb.Create("bdb", wallet.FilePath)
	if err != nil {
		return nil, err
	}

	wallet.DB = db
	wallets[name] = wallet

	// حفظ المحفظة في الذاكرة
	if err := SaveWallet(wallet); err != nil {
		return nil, err
	}

	return wallet, nil
}

func SaveWallet(wallet *Wallet) error {
	wallet.mu.Lock()
	defer wallet.mu.Unlock()

	// في نظام حقيقي، هنا ننفذ حفظ البيانات في قاعدة البيانات
	return nil
}

func GenerateNewAddress(wallet *Wallet) (string, error) {
	// توليد عنوان جديد (هذا مثال مبسط)
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "1" + hex.EncodeToString(b)[:33], nil
}

// ==================== وظائف المعاملات ====================

func CreateTransaction(from, to string, amount float64) (*Transaction, error) {
	tx := &Transaction{
		ID:        hex.EncodeToString(make([]byte, 16)),
		From:      from,
		To:        to,
		Amount:    amount,
		Fee:       0.0001, // رسوم ثابتة كمثال
		Timestamp: time.Now(),
		Status:    "Pending",
	}

	// هنا نضيف المنطق الحقيقي لإنشاء المعاملة على الشبكة
	transactions = append(transactions, *tx)
	return tx, nil
}

// ==================== خدمة أسعار البيتكوين ====================

func StartPriceService() {
	ticker := time.NewTicker(3 * time.Minute)
	go func() {
		for {
			updatePrice()
			<-ticker.C
		}
	}()
}

func updatePrice() {
	resp, err := http.Get("https://api.coindesk.com/v1/bpi/currentprice.json")
	if err != nil {
		fmt.Println("Error fetching price:", err)
		return
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Println("Error decoding price:", err)
		return
	}

	if bpi, ok := result["bpi"].(map[string]interface{}); ok {
		if usd, ok := bpi["USD"].(map[string]interface{}); ok {
			if rate, ok := usd["rate_float"].(float64); ok {
				priceService.CurrentPrice = rate
				priceService.LastUpdated = time.Now()
				updatePriceLabel()
			}
		}
	}
}

// ==================== الواجهة الرسومية ====================

func initGUI() {
	gtk.Init(nil)

	// إنشاء النافذة الرئيسية
	win, err := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		panic(err)
	}
	mainWindow = win
	win.SetTitle("Bitcoin Wallet Manager")
	win.SetDefaultSize(800, 600)
	win.Connect("destroy", func() {
		gtk.MainQuit()
	})

	// إنشاء التخطيط الرئيسي
	box, err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 5)
	if err != nil {
		panic(err)
	}

	// شريط العنوان
	header, err := gtk.HeaderBarNew()
	if err != nil {
		panic(err)
	}
	header.SetShowCloseButton(true)
	header.SetTitle("Bitcoin Wallet Manager")
	win.SetTitlebar(header)

	// زر إنشاء محفظة جديدة
	createBtn, err := gtk.ButtonNewWithLabel("Create New Wallet")
	if err != nil {
		panic(err)
	}
	createBtn.Connect("clicked", createWalletDialog)
	header.PackStart(createBtn)

	// قسم عرض السعر
	priceBox, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 5)
	if err != nil {
		panic(err)
	}

	priceLbl, err := gtk.LabelNew("BTC Price: $")
	if err != nil {
		panic(err)
	}
	priceLabel, err = gtk.LabelNew("Loading...")
	if err != nil {
		panic(err)
	}
	priceBox.PackStart(priceLbl, false, false, 5)
	priceBox.PackStart(priceLabel, false, false, 5)
	box.PackStart(priceBox, false, false, 5)

	// قائمة المحافظ
	listBox, err := gtk.ListBoxNew()
	if err != nil {
		panic(err)
	}
	walletListBox = listBox
	box.PackStart(listBox, true, true, 5)

	// شريط الحالة
	statusLbl, err := gtk.LabelNew("Ready")
	if err != nil {
		panic(err)
	}
	statusLabel = statusLbl
	box.PackStart(statusLbl, false, false, 5)

	win.Add(box)
	win.ShowAll()
}

func updatePriceLabel() {
	if priceLabel != nil {
		priceLabel.SetText(fmt.Sprintf("$%.2f", priceService.CurrentPrice))
	}
}

func createWalletDialog() {
	dialog, err := gtk.DialogNew()
	if err != nil {
		panic(err)
	}
	dialog.SetTitle("Create New Wallet")

	contentArea, err := dialog.GetContentArea()
	if err != nil {
		panic(err)
	}

	entry, err := gtk.EntryNew()
	if err != nil {
		panic(err)
	}
	entry.SetPlaceholderText("Wallet Name")
	contentArea.Add(entry)

	dialog.AddButton("Create", gtk.RESPONSE_OK)
	dialog.AddButton("Cancel", gtk.RESPONSE_CANCEL)
	dialog.ShowAll()

	response := dialog.Run()
	if response == gtk.RESPONSE_OK {
		name, err := entry.GetText()
		if err == nil && name != "" {
			wallet, err := CreateWallet(name)
			if err != nil {
				showErrorDialog("Error creating wallet: " + err.Error())
			} else {
				addWalletToList(wallet)
				statusLabel.SetText("Created wallet: " + name)
			}
		}
	}
	dialog.Destroy()
}

func addWalletToList(wallet *Wallet) {
	row, err := gtk.ListBoxRowNew()
	if err != nil {
		panic(err)
	}

	box, err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 5)
	if err != nil {
		panic(err)
	}

	label, err := gtk.LabelNew(wallet.Name)
	if err != nil {
		panic(err)
	}
	box.PackStart(label, true, true, 5)

	addrBtn, err := gtk.ButtonNewWithLabel("Get Address")
	if err != nil {
		panic(err)
	}
	addrBtn.Connect("clicked", func() {
		addr, err := GenerateNewAddress(wallet)
		if err != nil {
			showErrorDialog("Error generating address: " + err.Error())
		} else {
			showInfoDialog("New Address", "Your new address: "+addr)
		}
	})
	box.PackStart(addrBtn, false, false, 5)

	sendBtn, err := gtk.ButtonNewWithLabel("Send BTC")
	if err != nil {
		panic(err)
	}
	sendBtn.Connect("clicked", func() {
		createSendDialog(wallet)
	})
	box.PackStart(sendBtn, false, false, 5)

	row.Add(box)
	walletListBox.Add(row)
	walletListBox.ShowAll()
}

func createSendDialog(wallet *Wallet) {
	dialog, err := gtk.DialogNew()
	if err != nil {
		panic(err)
	}
	dialog.SetTitle("Send Bitcoin")

	contentArea, err := dialog.GetContentArea()
	if err != nil {
		panic(err)
	}

	grid, err := gtk.GridNew()
	if err != nil {
		panic(err)
	}
	grid.SetRowSpacing(5)
	grid.SetColumnSpacing(5)

	// حقل العنوان
	toLbl, err := gtk.LabelNew("To Address:")
	if err != nil {
		panic(err)
	}
	grid.Attach(toLbl, 0, 0, 1, 1)

	toEntry, err := gtk.EntryNew()
	if err != nil {
		panic(err)
	}
	toEntry.SetPlaceholderText("Recipient Address")
	grid.Attach(toEntry, 1, 0, 1, 1)

	// حقل المبلغ
	amountLbl, err := gtk.LabelNew("Amount:")
	if err != nil {
		panic(err)
	}
	grid.Attach(amountLbl, 0, 1, 1, 1)

	amountEntry, err := gtk.EntryNew()
	if err != nil {
		panic(err)
	}
	amountEntry.SetPlaceholderText("0.00")
	grid.Attach(amountEntry, 1, 1, 1, 1)

	contentArea.Add(grid)

	dialog.AddButton("Send", gtk.RESPONSE_OK)
	dialog.AddButton("Cancel", gtk.RESPONSE_CANCEL)
	dialog.ShowAll()

	response := dialog.Run()
	if response == gtk.RESPONSE_OK {
		toAddr, _ := toEntry.GetText()
		amountStr, _ := amountEntry.GetText()

		amount, err := strconv.ParseFloat(amountStr, 64)
		if err != nil {
			showErrorDialog("Invalid amount")
		} else if toAddr == "" {
			showErrorDialog("Recipient address is required")
		} else {
			_, err := CreateTransaction(wallet.Name, toAddr, amount)
			if err != nil {
				showErrorDialog("Error creating transaction: " + err.Error())
			} else {
				showInfoDialog("Success", "Transaction created successfully")
			}
		}
	}
	dialog.Destroy()
}

func showErrorDialog(msg string) {
	dialog := gtk.MessageDialogNew(mainWindow, gtk.DIALOG_MODAL, gtk.MESSAGE_ERROR, gtk.BUTTONS_OK, msg)
	dialog.Run()
	dialog.Destroy()
}

func showInfoDialog(title, msg string) {
	dialog := gtk.MessageDialogNew(mainWindow, gtk.DIALOG_MODAL, gtk.MESSAGE_INFO, gtk.BUTTONS_OK, msg)
	dialog.SetTitle(title)
	dialog.Run()
	dialog.Destroy()
}

// ==================== الوظيفة الرئيسية ====================

func main() {
	// تهيئة الواجهة الرسومية
	initGUI()

	// بدء خدمة تحديث الأسعار
	StartPriceService()

	// تحميل المحافظ الموجودة (في نظام حقيقي نقرأ من قاعدة البيانات)
	exampleWallet, _ := CreateWallet("Example Wallet")
	addWalletToList(exampleWallet)

	// تشغيل الواجهة
	gtk.Main()
}