window.addEventListener('load', () => {
  document.querySelector('#drag-start-area').addEventListener(
      'dragstart', (event) => {
    event.dataTransfer.setData('text/plain', 'Data text');
  });
}, {once: true});

