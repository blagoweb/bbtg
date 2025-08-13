package telegram

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "errors"
    "fmt"
    "github.com/dgrijalva/jwt-go"
    "net/url"
    "sort"
    "strings"
    "time"
)

// CheckAuthData проверяет подпись initData из Telegram WebApp
func CheckAuthData(initData string, botToken string) (map[string]string, error) {
    // Парсим строку вида "key1=value1&key2=value2..."
    vals, err := url.ParseQuery(initData)
    if err != nil {
        return nil, err
    }
    
    // Проверяем наличие hash или signature
    receivedHash := vals.Get("hash")
    if receivedHash == "" {
        receivedHash = vals.Get("signature")
        if receivedHash == "" {
            return nil, errors.New("missing hash or signature in initData")
        }
    }
    
    // Формируем data_check_string
    var keys []string
    for k := range vals {
        if k == "hash" || k == "signature" {
            continue
        }
        keys = append(keys, k)
    }
    sort.Strings(keys)
    var dataStrings []string
    for _, k := range keys {
        dataStrings = append(dataStrings, fmt.Sprintf("%s=%s", k, vals.Get(k)))
    }
    dataCheckString := strings.Join(dataStrings, "\n")

    // Вычисляем секретный ключ как HMAC-SHA256 от botToken
    secretKey := sha256.Sum256([]byte(botToken))

    // Считаем HMAC от строки данных
    mac := hmac.New(sha256.New, secretKey[:])
    mac.Write([]byte(dataCheckString))
    expectedHash := hex.EncodeToString(mac.Sum(nil))

    if !hmac.Equal([]byte(expectedHash), []byte(receivedHash)) {
        // Добавляем отладочную информацию
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

    // Собираем результат в map
    result := make(map[string]string)
    for _, k := range keys {
        result[k] = vals.Get(k)
    }
    return result, nil
}

// GenerateJWT создаёт JWT-токен с user_id и username из данных Telegram
func GenerateJWT(data map[string]string, jwtSecret string) (string, error) {
    userID := data["user_id"]
    username := data["username"]

    claims := jwt.MapClaims{
        "user_id":  userID,
        "username": username,
        "exp":      time.Now().Add(24 * time.Hour).Unix(),
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(jwtSecret))
}