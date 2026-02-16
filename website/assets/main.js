const setYear = () => {
  const el = document.getElementById('year')
  if (!el) return
  el.textContent = String(new Date().getFullYear())
}

const enableSmoothAnchors = () => {
  const links = document.querySelectorAll('a[href^="#"]')
  for (const link of links) {
    link.addEventListener('click', (e) => {
      const href = link.getAttribute('href')
      if (!href || href === '#') return
      const target = document.querySelector(href)
      if (!target) return
      e.preventDefault()
      target.scrollIntoView({ behavior: 'smooth', block: 'start' })
      history.pushState(null, '', href)
    })
  }
}

setYear()
enableSmoothAnchors()
