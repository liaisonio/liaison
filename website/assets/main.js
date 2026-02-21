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

const enableRevealOnScroll = () => {
  const sections = document.querySelectorAll('.section--reveal')
  if (!sections.length) return
  const observer = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (entry.isIntersecting) {
          entry.target.classList.add('revealed')
        }
      }
    },
    { rootMargin: '0px 0px -8% 0px', threshold: 0 }
  )
  sections.forEach((section) => observer.observe(section))
}

setYear()
enableSmoothAnchors()
enableRevealOnScroll()
