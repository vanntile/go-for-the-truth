const download = (name, obj) => {
  const url = window.URL.createObjectURL(new Blob([obj], { type: "plain/text" }));
  const l = document.createElement("a");
  l.style.display = "none";
  l.href = url;
  l.download = name;
  document.body.appendChild(l);
  l.click();
  l.remove();
  window.URL.revokeObjectURL(url);
};

window.addEventListener("DOMContentLoaded", (_event) => {
  const errorToast = document.querySelector("#error");

  const showError = (message) => {
    errorToast.children[0].textContent = `${message} Please reload or try again later.`;
    errorToast.classList.remove("hidden");
  };

  let downloadingQuestions = false;
  document.querySelector("#downloadQuestions").addEventListener("click", async () => {
    if (downloadingQuestions) return;
    downloadingQuestions = true;

    let questionStr = "ID,claims,fake,question\n";
    for (let question of document.querySelector("tbody").children) {
      const row = [];
      for (let field of question.children) {
        row.push(field.textContent);
      }
      questionStr += `${row[0]},${row[1]},${row[2]},"${row[3]}"\n`;
    }

    download("questions.csv", questionStr);
    downloadingQuestions = true;
  });

  let downloadingAnswers = false;
  document.querySelector("#downloadAnswers").addEventListener("click", async () => {
    if (downloadingAnswers) return;
    downloadingAnswers = true;

    let answers = "created,seed,country,side,age,answeredFake,answeredReal\n";

    let created;
    let callCount = 60; // 60 * 4e3 = 240e3
    while (callCount > 0) {
      callCount--;

      const res = await fetch(`/admin/answers${created ? "?created=" + encodeURIComponent(created) : ""}`);
      if (res.status != 200) {
        console.error(`failed to get answers (status = ${res.status})`);
      }

      const data = await res.text();
      if (data.length == 0) {
        break;
      }

      const isOnlyRow = data.lastIndexOf("\n20") == -1;
      const lastRow = data.substring(data.lastIndexOf("\n20"));
      created = lastRow.substring(isOnlyRow ? 0 : 1, lastRow.indexOf(","));
      answers += data;
      if (data.split("\n").length < 4000) {
        break;
      }
    }

    download("answers.csv", answers);
    downloadingAnswers = false;
  });

  document.querySelector("#uploadQuestions").addEventListener(
    "change",
    async function () {
      const files = this.files;
      if (files.length == 0) return;

      const formData = new FormData();
      formData.append("file", files[0]);
      const res = await fetch("/admin/questions", {
        method: "POST",
        body: formData,
      });
      if (res.status == 200) {
        window.location.reload();
      } else {
        showError("Failed to upload questions.");
      }
    },
    false,
  );
});
