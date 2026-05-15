/**
 * app.js — FluxHUB Vite entry point
 *
 * Vite bundles this file (+ all imports) into static/dist/app.js.
 * CSS imports are extracted by Vite into static/dist/app.css.
 *
 * Alpine.js is started here — do NOT load it from a CDN or <script> tag.
 */

import Alpine from 'alpinejs'
import hljs from 'highlight.js/lib/core'

// Highlight.js languages used in the file browser & diff viewer
import php        from 'highlight.js/lib/languages/php'
import javascript from 'highlight.js/lib/languages/javascript'
import css        from 'highlight.js/lib/languages/css'
import json       from 'highlight.js/lib/languages/json'
import xml        from 'highlight.js/lib/languages/xml'
import markdown   from 'highlight.js/lib/languages/markdown'
import sql        from 'highlight.js/lib/languages/sql'
import bash       from 'highlight.js/lib/languages/bash'
import yaml       from 'highlight.js/lib/languages/yaml'
import plaintext  from 'highlight.js/lib/languages/plaintext'

// App CSS (Tailwind + @layer components + highlight.js themes)
import '../css/app.css'

// Alpine components
import { fileBrowser } from './file-browser.js'
import { diffViewer }  from './diff-viewer.js'
import { reviewApp }   from './review-app.js'
import { showToast }   from './utils.js'

// ----------------------------------------------------------------
// Register highlight.js languages (tree-shaken — only what we use)
// ----------------------------------------------------------------
hljs.registerLanguage('php',        php)
hljs.registerLanguage('javascript', javascript)
hljs.registerLanguage('css',        css)
hljs.registerLanguage('json',       json)
hljs.registerLanguage('xml',        xml)
hljs.registerLanguage('markdown',   markdown)
hljs.registerLanguage('sql',        sql)
hljs.registerLanguage('bash',       bash)
hljs.registerLanguage('yaml',       yaml)
hljs.registerLanguage('plaintext',  plaintext)

// Expose hljs globally — used inside fileBrowser and diffViewer
// (their JS-generated HTML calls hljs.highlight / hljs.highlightElement)
window.hljs = hljs

// ----------------------------------------------------------------
// Register Alpine components
// Alpine.data() supports arguments: x-data="fileBrowser('id', ...)"
// ----------------------------------------------------------------
Alpine.data('fileBrowser', fileBrowser)
Alpine.data('diffViewer',  diffViewer)
Alpine.data('reviewApp',   reviewApp)

// Expose showToast globally — called from toast-container
window.showToast = showToast

// ----------------------------------------------------------------
// Start Alpine
// ----------------------------------------------------------------
window.Alpine = Alpine
Alpine.start()
