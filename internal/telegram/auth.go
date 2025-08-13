package telegram

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json" // FIX: для парсинга user
    "errors"
    "fmt"
    "net/url"
    "sort"
    "strings"
    "time"
)

type tgUser struct { // FIX: структура для user
    ID        int64  `json:"id"`
    Username  string `json:"username"`
    FirstName string `json:"first_name"`
    LastName  string `json:"last_name"`
}

func CheckAuthData(initData string, botToken string) (map[string]string, error) {
    // ParseQuery сам сделает URL-decode каждой пары
    vals, err := url.ParseQuery(initData)
    if err != nil {
        return nil, err
    }

    // Берём hash (а signature игнорируем по спекам)
    receivedHash := strings.ToLower(vals.Get("hash")) // FIX: lower
    if receivedHash == "" {
        return nil, errors.New("missing hash in initData")
    }

    // Собираем пары без hash и signature
    var keys []string
    for k := range vals {
        if k == "hash" || k == "signature" { // FIX: signature исключаем
            continue
        }
        keys = append(keys, k)
    }
    sort.Strings(keys)

    var dataStrings []string
    for _, k := range keys {
        // Важно: брать ровно то, что пришло (ParseQuery уже декодировал)
        dataStrings = append(dataStrings, fmt.Sprintf("%s=%s", k, vals.Get(k)))
    }
    dataCheckString := strings.Join(dataStrings, "\n")

    // FIX: secret_key = HMAC_SHA256(botToken, key="WebAppData")
    secretMac := hmac.New(sha256.New, []byte("WebAppData"))
    secretMac.Write([]byte(botToken))
    secretKey := secretMac.Sum(nil)

    // calc_hash = HMAC_SHA256(data_check_string, key=secret_key)
    mac := hmac.New(sha256.New, secretKey)
    mac.Write([]byte(dataCheckString))
    expectedHash := strings.ToLower(hex.EncodeToString(mac.Sum(nil))) // FIX: lower

    if !hmac.Equal([]byte(expectedHash), []byte(receivedHash)) {
        fmt.Printf("Debug info:\n")
        fmt.Printf("  Data check string: %s\n", dataCheckString)
        if len(botToken) > 10 {
            fmt.Printf("  Bot token (first 10 chars): %s...\n", botToken[:10])
        } else {
            fmt.Printf("  Bot token: %s\n", botToken)
        }
        fmt.Printf("  Expected hash: %s\n", expectedHash)
        fmt.Printf("  Received hash: %s\n", receivedHash)
        return nil, fmt.Errorf("hash mismatch: expected %s, received %s", expectedHash, receivedHash)
    }

    // Готовим результат
    result := make(map[string]string, len(keys)+2)
    for _, k := range keys {
        result[k] = vals.Get(k)
    }

    // FIX: распарсим user и положим user_id/username для GenerateJWT
    if userRaw := vals.Get("user"); userRaw != "" {
        var u tgUser
        if err := json.Unmarshal([]byte(userRaw), &u); err == nil {
            result["user_id"] = fmt.Sprint(u.ID)
            if u.Username != "" {
                result["username"] = u.Username
            } else if fn := strings.TrimSpace(u.FirstName + " " + u.LastName); fn != "" {
                result["username"] = fn
            }
        }
    }

    return result, nil
}