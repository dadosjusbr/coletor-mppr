package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dadosjusbr/status"
)

const (
	defaultGeneralTimeout      = 4 * time.Minute  // Duração máxima total da coleta de todos os arquivos. Valor padrão calculado a partir de uma média de execuções ~4.5min
	defaulTimeBetweenSteps     = 5 * time.Second  //	 Tempo de espera entre passos do coletor."
	defaultFileDownloadTimeout = 20 * time.Second // Duração que o coletor deve esperar até que o download de cada um dos arquivos seja concluído
)

func main() {
	if _, err := strconv.Atoi(os.Getenv("MONTH")); err != nil {
		status.ExitFromError(status.NewError(status.InvalidInput, fmt.Errorf("invalid month (\"%s\"): %w", os.Getenv("MONTH"), err)))
	}
	month := os.Getenv("MONTH")

	if _, err := strconv.Atoi(os.Getenv("YEAR")); err != nil {
		status.ExitFromError(status.NewError(status.InvalidInput, fmt.Errorf("invalid year (\"%s\"): %w", os.Getenv("YEAR"), err)))
	}
	year := os.Getenv("YEAR")

	outputFolder := os.Getenv("OUTPUT_FOLDER")
	if outputFolder == "" {
		outputFolder = "/output"
	}

	if err := os.Mkdir(outputFolder, os.ModePerm); err != nil && !os.IsExist(err) {
		status.ExitFromError(status.NewError(status.SystemError, fmt.Errorf("error creating output folder(%s): %w", outputFolder, err)))
	}

	monthConverted, err := strconv.Atoi(month)
	if err != nil {
		status.ExitFromError(status.NewError(status.InvalidInput, fmt.Errorf("error converting month to int: %w", err)))
	}
	yearConverted, err := strconv.Atoi(year)
	if err != nil {
		status.ExitFromError(status.NewError(status.InvalidInput, fmt.Errorf("error converting year to int: %w", err)))
	}

	var downloads []string

	// Até maio de 2023 os dados poderiam ser baixados de forma direta, após isso faz-se necessário a simulação de usuário
	if yearConverted < 2023 || yearConverted == 2023 && monthConverted <= 5 {
		// Download de contracheques
		cLink := fmt.Sprintf("http://apps.mppr.mp.br/planilhas_transparencia/mptransp%s%smafp.ods", year, month)
		cName := filepath.Join(outputFolder, fmt.Sprintf("membros-ativos-contracheque-%s-%s.ods", month, year))
		if err := download(cLink, cName); err != nil {
			status.ExitFromError(err)
		}
		downloads = append(downloads, cName)
		// Download das verbas indenizatórias
		iLink := fmt.Sprintf("http://apps.mppr.mp.br/planilhas_transparencia/mptransp%s%smavio.ods", year, month)
		iName := filepath.Join(outputFolder, fmt.Sprintf("membros-ativos-verbas-indenizatorias-%s-%s.ods", month, year))
		if err := download(iLink, iName); err != nil {
			status.ExitFromError(err)
		}
		downloads = append(downloads, iName)
	} else {
		generalTimeout := defaultGeneralTimeout
		if os.Getenv("GENERAL_TIMEOUT") != "" {
			var err error
			generalTimeout, err = time.ParseDuration(os.Getenv("GENERAL_TIMEOUT"))
			if err != nil {
				log.Fatalf("Invalid GENERAL_TIMEOUT (\"%s\"): %q", os.Getenv("GENERAL_TIMEOUT"), err)
			}
		}

		timeBetweenSteps := defaulTimeBetweenSteps
		if os.Getenv("TIME_BETWEEN_STEPS") != "" {
			var err error
			timeBetweenSteps, err = time.ParseDuration(os.Getenv("TIME_BETWEEN_STEPS"))
			if err != nil {
				log.Fatalf("Invalid TIME_BETWEEN_STEPS (\"%s\"): %q", os.Getenv("TIME_BETWEEN_STEPS"), err)
			}
		}

		downloadTimeout := defaultFileDownloadTimeout
		if os.Getenv("DOWNLOAD_TIMEOUT") != "" {
			var err error
			downloadTimeout, err = time.ParseDuration(os.Getenv("DOWNLOAD_TIMEOUT"))
			if err != nil {
				status.ExitFromError(status.NewError(status.InvalidInput, fmt.Errorf("invalid TIME_BETWEEN_STEPS (\"%s\"): %w", os.Getenv("TIME_BETWEEN_STEPS"), err)))
			}
		}

		c := crawler{
			generalTimeout:   generalTimeout,
			timeBetweenSteps: timeBetweenSteps,
			downloadTimeout:  downloadTimeout,
			year:             year,
			month:            month,
			output:           outputFolder,
		}

		downloads, err = c.crawl()
		if err != nil {
			status.ExitFromError(status.NewError(status.OutputError, fmt.Errorf("error crawling (%s, %s, %s): %w", year, month, outputFolder, err)))
		}

	}
	// O parser do MPPR espera os arquivos separados por \n.
	fmt.Println(strings.Join(downloads, "\n"))
}
