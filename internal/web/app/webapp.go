package app

import (
	_ "embed"
	"github.com/monopole/mdrip/v2/internal/web/app/widget/common"
	"github.com/monopole/mdrip/v2/internal/web/app/widget/mdrip"
	"github.com/monopole/mdrip/v2/internal/web/config"
)

const (
	TmplName = "tmplWebApp"

	MimeJs  = "application/javascript"
	MimeCss = "text/css"

	// classlessCss = "https://cdn.jsdelivr.net/npm/water.css@2/out/dark.css"
	// classlessCss = "https://raw.githubusercontent.com/raj457036/attriCSS/master/themes/darkforest-green.css"
	//
	//   To load some "classless" css, throw in a line like
	//     <link rel="stylesheet" href="` + classlessCss + `">
	//   There are many classless css examples at
	//      https://github.com/dbohdan/classless-css
	//   Most of them mess with <body> and <pre>, screwing up mdrip's layout,
	//   but maybe copy a subset.
)

var (
	// Don't forget to set the content-type header if you use this.
	cssViaLink = `<link rel='stylesheet' type='` + MimeCss +
		`' href='` + config.Dynamic(config.RouteCss) + `' />`

	// Use this instead of cssViaLink to inject directly into the html response.
	cssInjected = `<style> ` + mdrip.AllCss + ` </style>`
)

var (
	html = `
<!DOCTYPE html>
<html lang="en">
  <head>
    <title>{{.AppState.Title}}</title>
    ` + cssViaLink + `
    <script type='` + MimeJs + `' src='` + config.Dynamic(config.RouteJs) + `'></script>
    <script type='` + MimeJs + `'>
      function makeEmptyCache() {
        let c = new Array({{len .AppState.RenderedFiles}});
        for (let i = 0; i < c.length; i++) {
          c[i] = null;
        }
        return c;
      }
      // Define these outside onLoad to allow console access (debugging).
      let sc = null;
      let as = null;
      let nac = null;
      let cellCounter = 0;
      const codeCellsContainerId = 'code-cells-container';

      function createCellHTML(cellId) {
        return `
          <div class="interactive-code-cell" id="cell-${cellId}" style="border: 1px solid #eee; margin-bottom: 10px; padding: 10px;">
            <h5>Cell ${cellId}</h5>
            <textarea id="code-input-${cellId}" rows="5" style="width: 98%;" placeholder="Enter shell command..."></textarea>
            <br>
            <button id="run-code-button-${cellId}" class="run-button">Run</button>
            <button id="remove-cell-button-${cellId}" class="remove-button">Remove</button>
            <h6>Standard Output:</h6>
            <pre id="stdout-output-${cellId}" style="border: 1px solid #ccc; background-color: #f8f8f8; padding: 5px; min-height: 30px;"></pre>
            <h6>Standard Error:</h6>
            <pre id="stderr-output-${cellId}" style="border: 1px solid #ccc; background-color: #f8f8f8; padding: 5px; min-height: 30px; color: red;"></pre>
          </div>
        `;
      }

      function executeCodeInCell(cellId) {
        const codeInput = document.getElementById(`code-input-${cellId}`);
        const stdoutOutput = document.getElementById(`stdout-output-${cellId}`);
        const stderrOutput = document.getElementById(`stderr-output-${cellId}`);
        const command = codeInput.value;

        stdoutOutput.textContent = 'Executing...';
        stderrOutput.textContent = '';

        fetch('` + config.Dynamic(config.RouteRunBlock) + `', {
          method: 'POST',
          headers: { 'Content-Type': 'text/plain' },
          body: command,
        })
        .then(response => {
          if (!response.ok) {
            return response.text().then(text => {
              throw new Error(`HTTP error ${response.status}: ${text || response.statusText}`);
            });
          }
          return response.json();
        })
        .then(data => {
          stdoutOutput.textContent = data.stdout || '';
          stderrOutput.textContent = data.stderr || '';
          if (data.error) {
            stderrOutput.textContent += (stderrOutput.textContent ? '\n' : '') + 'Execution Error: ' + data.error;
          }
        })
        .catch(error => {
          stdoutOutput.textContent = ''; // Clear "Executing..." message
          stderrOutput.textContent = 'Error: ' + error.message;
          console.error('Fetch operation error for cell ' + cellId + ':', error);
        });
      }

      function attachCellEventListeners(cellId) {
        const runButton = document.getElementById(`run-code-button-${cellId}`);
        const removeButton = document.getElementById(`remove-cell-button-${cellId}`);

        if (runButton) {
          runButton.addEventListener('click', () => executeCodeInCell(cellId));
        } else {
          console.error(`Run button not found for cell ${cellId}`);
        }

        if (removeButton) {
          removeButton.addEventListener('click', () => {
            const cellElement = document.getElementById(`cell-${cellId}`);
            if (cellElement) {
              cellElement.remove();
            } else {
              console.error(`Cell element not found for removal: cell-${cellId}`);
            }
          });
        } else {
          console.error(`Remove button not found for cell ${cellId}`);
        }
      }

      function addCodeCell() {
        cellCounter++;
        const newCellHTML = createCellHTML(cellCounter);
        const cellsContainer = document.getElementById(codeCellsContainerId);
        if (cellsContainer) {
          const cellWrapper = document.createElement('div');
          cellWrapper.innerHTML = newCellHTML;
          // innerHTML creates the div "interactive-code-cell" itself, so we append its first child
          if (cellWrapper.firstChild) {
            cellsContainer.appendChild(cellWrapper.firstChild);
            attachCellEventListeners(cellCounter);
          } else {
             console.error("Could not create cell HTML structure correctly.");
          }
        } else {
          console.error(`Container with id "${codeCellsContainerId}" not found.`);
        }
      }

      function onLoad() {
        sc = new SessionController(makeEmptyCache());
        as = new AppState(sc, {{.AppState.InitialRender}});
        nac = new MdRipController(as);
        sc.enable();
        // Load the initial (zeroth) file.
        as.loadCurrentFile(StartAt.Top, ActivateBlock.No);

        const addCellButton = document.getElementById('add-code-cell-button');
        if (addCellButton) {
          addCellButton.addEventListener('click', addCodeCell);
        } else {
          console.error('Add code cell button not found.');
        }
        // Add one initial cell on load
        addCodeCell();
      }
    </script>
    <style>
      /* Basic styling for buttons, can be expanded or moved to cssInjected */
      .run-button, .remove-button, #add-code-cell-button {
        margin: 5px;
        padding: 8px 12px;
        border-radius: 4px;
        cursor: pointer;
      }
      #add-code-cell-button {
        background-color: #4CAF50; /* Green */
        color: white;
        border: none;
      }
      .run-button {
        background-color: #007bff; /* Blue */
        color: white;
        border: none;
      }
      .remove-button {
        background-color: #f44336; /* Red */
        color: white;
        border: none;
      }
    </style>
  </head>
  <body onload='onLoad()'>
  {{template "` + mdrip.TmplNameHtml + `" .}}

  <hr>
  <div style="padding: 1em;">
    <h3>Interactive Shell Cells</h3>
    <button id="add-code-cell-button">Add Code Cell (+)</button>
    <div id="code-cells-container" style="margin-top: 10px;">
      <!-- Dynamically added code cells will go here -->
    </div>
  </div>

  </body>
</html>
`
)

func AsTmpl() string {
	return mdrip.AsTmplHtml() + common.AsTmpl(TmplName, html)
}
