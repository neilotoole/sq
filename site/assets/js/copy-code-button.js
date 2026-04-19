// This script adds a "Copy" button to syntax highlight blocks.
// It was necessary to implement this because the base Doks
// theme uses highlight.js and clipboard.js. This site is
// instead using Chroma for syntax highlighting. This
// code below implements the Copy button.
//
// This impl is mostly lifted from:
//   https://digitaldrummerj.me/hugo-add-copy-code-snippet-button/
//
// However, this impl uses "btn btn-copy" classes to take advantage
// of the styling already available in the theme.
//
// See also: /assets/scss/_copy-button.scss
function createCopyButton(highlightDiv) {
  const button = document.createElement('button');
  button.className = 'btn btn-copy';
  button.type = 'button';
  button.addEventListener('click', () => copyCodeToClipboard(button, highlightDiv));

  const chromaDiv = highlightDiv.querySelector('pre.chroma')
  chromaDiv.insertBefore(button, chromaDiv.firstChild);
}

document.querySelectorAll('.highlight').forEach((highlightDiv) => createCopyButton(highlightDiv));

async function copyCodeToClipboard(button, highlightDiv) {
  const codeToCopy = highlightDiv.querySelector('.chroma > code').innerText;
  console.log(codeToCopy)
  try {
    const result = await navigator.permissions.query({name: 'clipboard-write'});
    if (result.state == 'granted' || result.state == 'prompt') {
      await navigator.clipboard.writeText(codeToCopy);
    } else {
      copyCodeBlockExecCommand(codeToCopy, highlightDiv);
    }
  } catch (_) {
    copyCodeBlockExecCommand(codeToCopy, highlightDiv);
  } finally {
    button.blur();
  }
}

function copyCodeBlockExecCommand(codeToCopy, highlightDiv) {
  const textArea = document.createElement('textArea');
  textArea.contentEditable = 'true';
  textArea.readOnly = 'false';
  textArea.className = 'copyable-text-area';
  textArea.value = codeToCopy;
  highlightDiv.insertBefore(textArea, highlightDiv.firstChild);
  const range = document.createRange();
  range.selectNodeContents(textArea);
  const sel = window.getSelection();
  sel.removeAllRanges();
  sel.addRange(range);
  textArea.setSelectionRange(0, 999999);
  document.execCommand('copy');
  highlightDiv.removeChild(textArea);
}
