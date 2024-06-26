package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/chromedp"
	"github.com/dadosjusbr/status"
)

func download(url, path string) error {
	resp, err := http.Get(url)
	if err != nil {
		return status.NewError(status.ConnectionError, err)
	}
	if resp.StatusCode == 404 {
		return status.NewError(status.DataUnavailable, fmt.Errorf("sem dados"))
	}
	defer resp.Body.Close()
	cFile, err := os.Create(path)
	if err != nil {
		return status.NewError(status.SystemError, err)
	}
	defer cFile.Close()
	cWriter := bufio.NewWriter(cFile)
	if _, err := io.Copy(cWriter, resp.Body); err != nil {
		return status.NewError(status.SystemError, err)
	}
	cWriter.Flush()
	return nil
}

type crawler struct {
	// Aqui temos os atributos e métodos necessários para realizar a coleta dos dados
	generalTimeout   time.Duration
	timeBetweenSteps time.Duration
	downloadTimeout  time.Duration
	year             string
	month            string
	output           string
}

func (c crawler) crawl() ([]string, error) {
	// Chromedp setup.
	log.SetOutput(os.Stderr) // Enviando logs para o stderr para não afetar a execução do coletor.
	alloc, allocCancel := chromedp.NewExecAllocator(
		context.Background(),
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/77.0.3830.0 Safari/537.36"),
			chromedp.Flag("headless", true), // mude para false para executar com navegador visível.
			chromedp.NoSandbox,
			chromedp.DisableGPU,
		)...,
	)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(
		alloc,
		chromedp.WithLogf(log.Printf), // remover comentário para depurar
	)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, c.generalTimeout)
	defer cancel()

	// Contracheques
	log.Printf("Selecionando contracheques (%s/%s)...", c.month, c.year)
	if err := c.selecionaContracheque(ctx); err != nil {
		status.ExitFromError(err)
	}
	log.Printf("Seleção realizada com sucesso!\n")

	log.Printf("Baixando contracheques (%s/%s)...", c.month, c.year)
	cName := filepath.Join(c.output, fmt.Sprintf("membros-ativos-contracheque-%s-%s.xls", c.month, c.year))
	if err := c.exportaExcel(ctx, cName); err != nil {
		status.ExitFromError(err)
	}
	log.Printf("Download de contracheques realizado com sucesso!\n")

	// Verbas Indenizatórias
	log.Printf("Selecionando verbas indenizatórias (%s/%s)...", c.month, c.year)
	if err := c.selecionaVerbasIndenizatorias(ctx); err != nil {
		status.ExitFromError(err)
	}
	log.Printf("Seleção realizada com sucesso!\n")

	log.Printf("Baixando verbas indenizatórias (%s/%s)...", c.month, c.year)
	iName := filepath.Join(c.output, fmt.Sprintf("membros-ativos-verbas-indenizatorias-%s-%s.xls", c.month, c.year))
	if err := c.exportaExcel(ctx, iName); err != nil {
		status.ExitFromError(err)
	}
	log.Printf("Download de verbas indenizatórias realizado com sucesso!\n")

	return []string{cName, iName}, nil
}

func (c crawler) selecionaContracheque(ctx context.Context) error {
	monthMap := map[string]string{
		"01": "Janeiro",
		"02": "Fevereiro",
		"03": "Marco",
		"04": "Abril",
		"05": "Maio",
		"06": "Junho",
		"07": "Julho",
		"08": "Agosto",
		"09": "Setembro",
		"10": "Outubro",
		"11": "Novembro",
		"12": "Dezembro",
	}

	return chromedp.Run(ctx,
		chromedp.Navigate("https://apps.mppr.mp.br/PortaleAdm/app/portalTransparencia"),
		chromedp.Sleep(c.timeBetweenSteps),
		// Seleciona o ano
		chromedp.Click(`//*[@id="formPortalTransparencia:idSelectAno"]`),
		chromedp.Sleep(c.timeBetweenSteps),
		chromedp.Click(fmt.Sprintf(`/html/body/div[7]/div[2]/ul/li[@data-label="%s"]`, c.year)),
		chromedp.Sleep(c.timeBetweenSteps),
		// Seleciona o mês
		chromedp.Click(`//*[@id="formPortalTransparencia:idSelectMes"]`),
		chromedp.Sleep(c.timeBetweenSteps),
		chromedp.Click(fmt.Sprintf(`/html/body/div[8]/div[2]/ul/li[@data-label="%s"]`, monthMap[c.month])),
		chromedp.Sleep(c.timeBetweenSteps),
		// Consulta membros ativos
		chromedp.Click(`//*[@id="formPortalTransparencia:tabViewPrincipal:j_idt42:j_idt45"]`),
		chromedp.Sleep(c.timeBetweenSteps),

		// Altera o diretório de download
		browser.SetDownloadBehavior(browser.SetDownloadBehaviorBehaviorAllowAndName).
			WithDownloadPath(c.output).
			WithEventsEnabled(true),
	)
}

func (c crawler) selecionaVerbasIndenizatorias(ctx context.Context) error {
	return chromedp.Run(ctx,
		// Retorna à página anterior
		chromedp.NavigateBack(),
		chromedp.Sleep(c.timeBetweenSteps),
		// Consulta verbas indenizatórias
		chromedp.Click(`//*[@id="formPortalTransparencia:tabViewPrincipal:j_idt42:j_idt71_header"]`),
		chromedp.Sleep(c.timeBetweenSteps),
		// Consulta membros ativos
		chromedp.Click(`//*[@id="formPortalTransparencia:tabViewPrincipal:j_idt42:j_idt73"]`),
		chromedp.Sleep(c.timeBetweenSteps),
	)
}

// exportaExcel clica no botão correto para exportar para excel, espera um tempo para download renomeia o arquivo.
func (c crawler) exportaExcel(ctx context.Context, fName string) error {
	complement := "FolhaPgtoMensal"
	if strings.Contains(fName, "indenizatorias") {
		complement = "VerbasIndenizatoriasOutrasRemuneracoesTemporarias"
	}

	if err := chromedp.Run(ctx,
		chromedp.Click(fmt.Sprintf(`//*[@id="formTranspFolhaPagamentoMensal:dttb%s"]/div[1]/div/a`, complement)),
		chromedp.Sleep(c.downloadTimeout),
	); err != nil {
		return status.NewError(status.ConnectionError, fmt.Errorf("erro clicando no botão de download: %v", err))
	}

	if err := nomeiaDownload(c.output, fName); err != nil {
		status.ExitFromError(err)
	}
	if _, err := os.Stat(fName); os.IsNotExist(err) {
		return status.NewError(status.DataUnavailable, fmt.Errorf("download do arquivo de %s não realizado", fName))
	}
	return nil
}

// nomeiaDownload dá um nome ao último arquivo modificado dentro do diretório
// passado como parâmetro nomeiaDownload dá pega um arquivo
func nomeiaDownload(output, fName string) error {
	// Identifica qual foi o ultimo arquivo
	files, err := os.ReadDir(output)
	if err != nil {
		return status.NewError(status.SystemError, fmt.Errorf("erro lendo diretório %s: %v", output, err))
	}
	var newestFPath string
	var newestTime int64 = 0
	for _, f := range files {
		fPath := filepath.Join(output, f.Name())
		fi, err := os.Stat(fPath)
		if err != nil {
			return status.NewError(status.SystemError, fmt.Errorf("erro obtendo informações sobre arquivo %s: %v", fPath, err))
		}
		currTime := fi.ModTime().Unix()
		if currTime > newestTime {
			newestTime = currTime
			newestFPath = fPath
		}
	}
	// Renomeia o ultimo arquivo modificado.
	if err := os.Rename(newestFPath, fName); err != nil {
		return status.NewError(status.DataUnavailable, fmt.Errorf("sem planilhas baixadas"))
	}
	return nil
}
