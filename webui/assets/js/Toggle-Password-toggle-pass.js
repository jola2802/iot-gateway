let tgBtns = document.querySelectorAll('.togglePassword')
tgBtns.forEach(tgBtn => {
    tgBtn.addEventListener("click", function(btn) {
        let pasBtn = btn.target.closest('.togglePassword')
        let passInput = pasBtn.previousSibling
        let faIcon = document.querySelector('#eye-icon')

        if (passInput.type == "password") {
            passInput.setAttribute('type', "text")
        } else {
            passInput.setAttribute('type', "password")
        }
        faIcon.classList.toggle('ion-eye-disabled')
        faIcon.classList.toggle('ion-eye')
    })
});