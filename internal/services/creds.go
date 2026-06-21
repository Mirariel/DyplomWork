package services

import (
	"github.com/okochadmytro/tradetracker/internal/models"
	"github.com/okochadmytro/tradetracker/internal/services/exchange"
)

// GetUserCreds знаходить і розшифровує API credentials користувача для вказаної біржі.
// Повертає exchange.ErrNoCredentials якщо активних credentials не знайдено.
func GetUserCreds(
	portfolio *models.PortfolioRepository,
	enc *EncryptionService,
	userID int64,
	exchangeName string,
) (exchange.Credentials, error) {
	creds, err := portfolio.GetCredentials(userID)
	if err != nil {
		return exchange.Credentials{}, err
	}
	for _, c := range creds {
		if c.Exchange != exchangeName || !c.IsActive {
			continue
		}
		apiKey, err := enc.Decrypt(c.ApiKeyEncrypted)
		if err != nil {
			return exchange.Credentials{}, err
		}
		apiSecret, err := enc.Decrypt(c.ApiSecretEncrypted)
		if err != nil {
			return exchange.Credentials{}, err
		}
		passphrase := ""
		if c.PassphraseEncrypted != nil {
			passphrase, _ = enc.Decrypt(*c.PassphraseEncrypted)
		}
		return exchange.Credentials{
			APIKey:     apiKey,
			APISecret:  apiSecret,
			Passphrase: passphrase,
		}, nil
	}
	return exchange.Credentials{}, exchange.ErrNoCredentials
}
