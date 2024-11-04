const DEFAULT_OPTION = "Select an option";
const US = "United States";
const ANIMATION_DELAY = 240; // ms

const togglePageVisible = (page, toVisible) => {
  if (toVisible) {
    page.classList.remove("hidden");
    page.classList.add("flex");
  } else {
    page.classList.add("hidden");
    page.classList.remove("flex");
  }
};

window.addEventListener("DOMContentLoaded", (_event) => {
  const quiz = document.querySelector("#quiz");
  const errorToast = document.querySelector("#error");

  const showError = (message) => {
    errorToast.children[0].textContent = `${message} Please reload or try again later.`;
    errorToast.classList.remove("hidden");
  };

  if (quiz.dataset.answers != undefined) {
    document.querySelector("#nextResults").addEventListener("click", () => {
      window.location.href = "";
    });
  } else {
    const count = parseInt(quiz.dataset.count, 10);

    // Element references
    const pageIntro = document.querySelector("#pageIntro");
    const nextIntro = document.querySelector("#nextIntro");
    const pageOutro = document.querySelector("#pageOutro");
    const nextOutro = document.querySelector("#nextOutro");
    const resultsLoading = document.querySelector("#resultsLoading");
    const pages = [];
    const fake = [];
    const real = [];

    for (let i = 0; i < count; i++) {
      try {
        pages.push(document.querySelector(`#page${i}`));
      } catch (err) {
        showError("Failed to load questions.");
        console.error(err);
        return;
      }
    }

    // Page navigation
    nextIntro.addEventListener("click", () => {
      setTimeout(() => {
        togglePageVisible(pages[0], true);
        togglePageVisible(pageIntro, false);
      }, ANIMATION_DELAY);
    });

    for (let i = 0; i < count; i++) {
      const fakeBtn = document.querySelector(`#fake${i}`);
      const realBtn = document.querySelector(`#real${i}`);
      const nextPage = (x) => {
        fakeBtn.disabled = "disabled";
        realBtn.disabled = "disabled";
        x.push(Number(pages[i].dataset.id));
        setTimeout(() => {
          togglePageVisible(i == count - 1 ? pageOutro : pages[i + 1], true);
          togglePageVisible(pages[i], false);
        }, ANIMATION_DELAY);
      };

      fakeBtn.addEventListener("click", () => {
        nextPage(fake);
      });
      realBtn.addEventListener("click", () => {
        nextPage(real);
      });
    }

    // Form
    const countryUSEl = document.querySelector("#countryUS");
    const formCountryEl = document.querySelector("#formCountry");
    const countrySelectEl = document.querySelector("#countrySelect");
    const sideSelectEl = document.querySelector("#sideSelect");
    const ageEl = document.querySelector("#age");
    const errCls = ["border-red-700", "text-red-700", "ring-red-700"];

    // Form reset and interaction
    if (localStorage.getItem("answer")) {
      try {
        const prevAnswer = JSON.parse(localStorage.getItem("answer"));
        if (prevAnswer.country == US) countryUSEl.value = "yes";
        else {
          countryUSEl.value = "no";
          countrySelectEl.value = prevAnswer.country;
          formCountryEl.classList.remove("hidden");
        }
        sideSelectEl.value = prevAnswer.side;
        ageEl.value = prevAnswer.age;
      } catch {}
    } else {
      [countryUSEl, countrySelectEl, sideSelectEl, ageEl].forEach((x) => {
        x.selectedIndex = 0;
      });
    }

    [countrySelectEl, sideSelectEl].forEach((e) => {
      e.addEventListener("input", () => {
        e.classList.remove(...errCls);
      });
    });

    countryUSEl.addEventListener("input", () => {
      countryUSEl.classList.remove(...errCls);
      if (countryUSEl.value == "no") formCountryEl.classList.remove("hidden");
      else formCountryEl.classList.add("hidden");
    });

    ageEl.addEventListener("input", () => {
      if (ageEl.value != "" && !isNaN(Number(ageEl.value))) {
        ageEl.classList.remove(...errCls);
      }
    });

    // Submitting results
    nextOutro.addEventListener("click", () => {
      let invalid = false;

      if (countryUSEl.value == DEFAULT_OPTION) {
        invalid = true;
        countryUSEl.classList.add(...errCls);
      } else if (countryUSEl.value == "no" && countrySelectEl.value == DEFAULT_OPTION) {
        invalid = true;
        countrySelectEl.classList.add(...errCls);
      }
      if (sideSelectEl.value == DEFAULT_OPTION) {
        invalid = true;
        sideSelectEl.classList.add(...errCls);
      }
      if (ageEl.value == "" || isNaN(Number(ageEl.value))) {
        invalid = true;
        ageEl.classList.add(...errCls);
      }

      if (invalid) return;

      const answer = {
        seed: quiz.dataset.seed,
        real: [...new Set(real)].sort((a, b) => a - b).join(","),
        fake: [...new Set(fake)].sort((a, b) => a - b).join(","),
        country: countryUSEl.value == "no" ? countrySelectEl.value : US,
        side: sideSelectEl.value,
        age: `${ageEl.value}`,
      };
      localStorage.setItem("answer", JSON.stringify(answer));

      nextOutro.classList.add("hidden");
      resultsLoading.classList.remove("hidden");

      const form = document.createElement("form");
      form.method = "POST";

      for (const key in answer) {
        if (answer.hasOwnProperty(key)) {
          const field = document.createElement("input");
          field.classList.add("hidden");
          field.type = "hidden";
          field.name = key;
          field.value = answer[key];

          form.appendChild(field);
        }
      }

      document.body.appendChild(form);
      form.submit();
    });
  }
});
