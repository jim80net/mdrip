// Global variables for cell management
let cellCounter = 0;
const codeCellsContainerId = 'code-cells-container';

// Function to create HTML for a new cell
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

// Function to execute code in a given cell
function executeCodeInCell(cellId) {
  const codeInput = document.getElementById(`code-input-${cellId}`);
  const stdoutOutput = document.getElementById(`stdout-output-${cellId}`);
  const stderrOutput = document.getElementById(`stderr-output-${cellId}`);
  
  if (!codeInput || !stdoutOutput || !stderrOutput) {
    console.error(`DOM elements for cell ${cellId} not found.`);
    return;
  }
  const command = codeInput.value;

  stdoutOutput.textContent = 'Executing...';
  stderrOutput.textContent = '';

  const runBlockURL = window.appConfig && window.appConfig.runBlockURL 
                      ? window.appConfig.runBlockURL 
                      : '/runblock'; // Fallback to '/runblock' if config is missing

  if (runBlockURL === '/runblock') {
    console.warn('Warning: Using fallback runBlockURL "/runblock". Check if window.appConfig.runBlockURL is correctly configured.');
  }
  
  fetch(runBlockURL, {
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

// Function to attach event listeners to a cell's buttons
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

// Function to add a new code cell
function addCodeCell() {
  cellCounter++;
  const newCellHTML = createCellHTML(cellCounter);
  const cellsContainer = document.getElementById(codeCellsContainerId);
  if (cellsContainer) {
    const cellWrapper = document.createElement('div');
    // Create a temporary parent to parse the newCellHTML
    // then append the actual cell element (the first child of the wrapper)
    cellWrapper.innerHTML = newCellHTML.trim(); 
    const cellElement = cellWrapper.firstChild;

    if (cellElement) {
      cellsContainer.appendChild(cellElement);
      attachCellEventListeners(cellCounter);
    } else {
       console.error("Could not create cell HTML structure correctly from: ", newCellHTML);
    }
  } else {
    console.error(`Container with id "${codeCellsContainerId}" not found.`);
  }
}

// Initialization function for interactive cells, to be called from onLoad in the main HTML
function initializeInteractiveCells() {
  if (!window.appConfig || !window.appConfig.runBlockURL) {
    console.error('Error: Application configuration (window.appConfig.runBlockURL) not found. Interactive cells might not work correctly or will use default endpoints.');
    // Optionally, disable UI elements if configuration is critical and missing.
    // For example, disable the "Add Code Cell" button:
    // const addCellBtn = document.getElementById('add-code-cell-button');
    // if (addCellBtn) addCellBtn.disabled = true;
  }

  const addCellButton = document.getElementById('add-code-cell-button');
  if (addCellButton) {
    addCellButton.addEventListener('click', addCodeCell);
  } else {
    console.error('Add code cell button not found.');
  }
  // Add one initial cell on load
  addCodeCell();
}

// Note: The original onLoad function in webapp.go will call initializeInteractiveCells()
// Ensure this script is loaded before onLoad is triggered if initializeInteractiveCells is called directly.
// Alternatively, ensure that the main onLoad calls this function.
